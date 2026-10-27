from pathlib import Path

import yaml
from conftest import logged_in
from openapi_schema_validator import OAS30Validator
from openapi_spec_validator import validate

SPEC = Path(__file__).parents[2] / "docs/openapi.yaml"


def test_shared_openapi_and_runtime_paths(ctx):
    spec = yaml.safe_load(SPEC.read_text())
    validate(spec)
    for path, item in ctx[1].openapi()["paths"].items():
        if not path.startswith("/api/"):
            continue
        assert path in spec["paths"]
        for method in item:
            assert method in spec["paths"][path]


def test_actual_identity_responses_match_shared_contract(ctx):
    headers = logged_in(ctx)
    spec = yaml.safe_load(SPEC.read_text())
    user = ctx[0].get("/api/users/me", headers=headers)
    OAS30Validator(spec["components"]["schemas"]["User"]).validate(user.json())
    profile = ctx[0].post(
        "/api/organizer-profile", headers=headers, json={"contactEmail": "owner@example.com"}
    )
    assert profile.status_code == 201
    OAS30Validator(spec["components"]["schemas"]["OrganizerProfile"]).validate(profile.json())
    error = ctx[0].get("/api/users/me")
    OAS30Validator(spec["components"]["schemas"]["Error"]).validate(error.json())


def test_oversize_credentials_rejected(ctx):
    response = ctx[0].post(
        "/api/auth/register", content=b"x" * 20000, headers={"Content-Type": "application/json"}
    )
    assert response.status_code == 413
