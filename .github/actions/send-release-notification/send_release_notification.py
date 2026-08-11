import boto3, hashlib, hmac, json, os, uuid, urllib.request
from botocore.auth import SigV4Auth
from botocore.awsrequest import AWSRequest

tag     = os.environ["GITHUB_REF_NAME"]
version = tag.lstrip("v")
repo    = os.environ["GITHUB_REPOSITORY"]

ghcr_prefix = os.environ["GHCR_PREFIX"]
target_ref  = f"{ghcr_prefix}:{version}"

source_ref = digest = None
with open(os.environ["DIGESTS_FILE"]) as f:
    for line in f:
        ref, dgst = line.strip().split()
        if ref == target_ref:
            source_ref, digest = ref, dgst
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

body        = json.dumps(payload, separators=(",", ":")).encode()
delivery_id = str(uuid.uuid4())
signature   = "sha256=" + hmac.new(os.environ["HMAC_SECRET"].encode(), body, hashlib.sha256).hexdigest()
url         = os.environ["API_URL"]

aws_req = AWSRequest(method="POST", url=url, data=body, headers={
    "Content-Type":            "application/json",
    "X-Release-Signature-256": signature,
    "X-Release-Delivery-Id":   delivery_id,
})
SigV4Auth(boto3.Session().get_credentials(), "execute-api", "eu-central-1").add_auth(aws_req)

req = urllib.request.Request(url, data=body, headers=dict(aws_req.headers), method="POST")
with urllib.request.urlopen(req) as resp:
    print(f"Ingestion API response: {resp.status}")
