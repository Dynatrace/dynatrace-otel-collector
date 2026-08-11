import boto3, hashlib, hmac, json, os, uuid, urllib3
from botocore.auth import SigV4Auth
from botocore.awsrequest import AWSRequest
from urllib3.util.retry import Retry

tag     = os.environ["GITHUB_REF_NAME"]
version = tag.lstrip("v")
repo    = os.environ["GITHUB_REPOSITORY"]

ghcr_prefix = os.environ["GHCR_PREFIX"]
target_ref  = f"{ghcr_prefix}:{version}"

source_ref = digest = None
with open(os.environ["DIGESTS_FILE"]) as f:
    for line in f:
        dgst, ref = line.strip().split()
        if ref == target_ref:
            source_ref, digest = ref, f"sha256:{dgst}"
            break

if not source_ref:
    raise RuntimeError(f"No GHCR manifest found for {target_ref} in {os.environ['DIGESTS_FILE']}")

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

http = urllib3.PoolManager(timeout=urllib3.Timeout(connect=5, read=30), retries=Retry(
    total=3,
    backoff_factor=1,
    backoff_jitter=1.0,
    status_forcelist={500, 502, 503, 504},
    allowed_methods={"POST"},
    raise_on_status=True,
))

resp = http.request("POST", url, body=body, headers=dict(aws_req.headers))
if resp.status >= 400:
    raise RuntimeError(f"Request failed with {resp.status}: {resp.data.decode()}")
print(f"Ingestion API response: {resp.status}")
