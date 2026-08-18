import base64, boto3, hashlib, hmac, json, os, uuid, urllib3
from botocore.auth import SigV4Auth
from botocore.awsrequest import AWSRequest
from urllib3.util.retry import Retry

tag     = os.environ["GITHUB_REF_NAME"]
version = tag.lstrip("v")
repo    = os.environ["GITHUB_REPOSITORY"]

ghcr_prefix = os.environ["GHCR_PREFIX"]
target_ref  = f"{ghcr_prefix}:{version}"

http = urllib3.PoolManager(timeout=urllib3.Timeout(connect=5, read=30), retries=Retry(
    total=3,
    backoff_factor=1,
    backoff_jitter=1.0,
    status_forcelist={500, 502, 503, 504},
    allowed_methods={"GET", "HEAD", "POST"},
    raise_on_status=True,
))

# Parse goreleaser's artifacts.json to find the manifest-list ref and all unique
# ghcr.io image digests that were signed.
#
# Relevant entry types:
#   "Published Docker Image" (internal_type 10) — per-arch images pushed to every registry;
#                                                  extra.Digest = content digest
#   "Docker Manifest"        (internal_type 11) — manifest lists; extra.Digest = content digest
#
# Both latest and versioned tags share the same digest, so deduplication is by digest.
artifacts_file = os.environ.get("ARTIFACTS_FILE", "dist/artifacts.json")
with open(artifacts_file) as f:
    goreleaser_artifacts = json.load(f)

source_ref = None
digest     = None
ghcr_hexes = set()   # unique image content-digest hex values on ghcr.io

for a in goreleaser_artifacts:
    a_type   = a.get("type", "")
    a_path   = a.get("path", "")
    a_digest = a.get("extra", {}).get("Digest", "")

    if a_type == "Docker Manifest" and a_path == target_ref:
        source_ref = a_path
        digest     = a_digest

    if a_type in ("Published Docker Image", "Docker Manifest") \
            and a_path.startswith(f"{ghcr_prefix}:") \
            and a_digest:
        ghcr_hexes.add(a_digest.removeprefix("sha256:"))

if not source_ref:
    raise RuntimeError(f"No manifest-list entry found for {target_ref} in {artifacts_file}")


def _ghcr_bearer_token(path: str) -> str:
    """Exchange a GitHub PAT for a short-lived ghcr.io pull token."""
    github_token = os.environ["GITHUB_TOKEN"]
    auth = base64.b64encode(f"token:{github_token}".encode()).decode()
    resp = http.request(
        "GET",
        f"https://ghcr.io/token?scope=repository:{path}:pull&service=ghcr.io",
        headers={"Authorization": f"Basic {auth}"},
    )
    return json.loads(resp.data)["token"]


def _ghcr_manifest_digest(path: str, tag: str, bearer: str) -> str:
    """Return the Docker-Content-Digest for a ghcr.io manifest tag."""
    resp = http.request(
        "HEAD",
        f"https://ghcr.io/v2/{path}/manifests/{tag}",
        headers={
            "Authorization": f"Bearer {bearer}",
            # Request the OCI image index media type — cosign stores the referrers
            # index (all sig bundles for a given image digest) as an OCI image index
            # under the tag sha256-<hex> (OCI referrers fallback schema).
            "Accept": "application/vnd.oci.image.index.v1+json,application/vnd.oci.image.manifest.v1+json,*/*",
        },
    )
    if resp.status != 200:
        raise RuntimeError(f"Failed to fetch manifest digest for {path}:{tag} — HTTP {resp.status}")
    return resp.headers["Docker-Content-Digest"]


# Build the signatures list: one OCI referrers-index ref per unique signed image digest.
#
# cosign 2.x stores signatures using the OCI referrers fallback tag schema: a tag named
# sha256-<image-hex> (no .sig suffix) that is an OCI image index whose children are the
# Sigstore bundle manifests (artifactType: application/vnd.dev.sigstore.bundle.v0.3+json).
#
# Promoting this OCI image index tag to ECR (with CopyImageOrIndex + PreserveDigests: true)
# copies the index and all the bundle manifests it references, making cosign verify work on
# the target registry without any separate signing step.
ghcr_path  = ghcr_prefix.removeprefix("ghcr.io/")
bearer     = _ghcr_bearer_token(ghcr_path)
signatures = []
for hex_dgst in sorted(ghcr_hexes):   # sorted for deterministic ordering
    ref_tag    = f"sha256-{hex_dgst}"
    idx_digest = _ghcr_manifest_digest(ghcr_path, ref_tag, bearer)
    signatures.append(f"{ghcr_prefix}:{ref_tag}@{idx_digest}")

component = os.environ["COMPONENT"]
payload = {
    "schema_version": "1",
    "component":      component,
    "version":        version,
    "github_release_url": f"https://github.com/{repo}/releases/tag/{tag}",
    "source_repo":       repo,
    "source_commit_sha": os.environ["GITHUB_SHA"],
    "artifacts": [{
        "type":       "docker-image",
        "name":       component,
        "source_ref": source_ref,
        "digest":     digest,
        "signatures": signatures,
    }],
}

body = json.dumps(payload, separators=(",", ":")).encode()

# Deterministic per run attempt so retries reuse the same ID and manual re-runs get a fresh one
delivery_id = str(uuid.uuid5(uuid.NAMESPACE_URL, f"{repo}/{tag}/{os.environ['GITHUB_RUN_ID']}/{os.environ['GITHUB_RUN_ATTEMPT']}"))
signature   = "sha256=" + hmac.new(os.environ["HMAC_SECRET"].encode(), body, hashlib.sha256).hexdigest()
url         = os.environ["API_URL"]
region      = os.environ["AWS_REGION"]

aws_req = AWSRequest(method="POST", url=url, data=body, headers={
    "Content-Type":            "application/json",
    "X-Release-Signature-256": signature,
    "X-Release-Delivery-Id":   delivery_id,
})
SigV4Auth(boto3.Session().get_credentials(), "execute-api", region).add_auth(aws_req)

resp = http.request("POST", url, body=body, headers=dict(aws_req.headers))
if resp.status >= 400:
    raise RuntimeError(f"Request failed with {resp.status}: {resp.data.decode()}")
print(f"Ingestion API response: {resp.status}")
