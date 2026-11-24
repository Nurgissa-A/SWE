"""Add/update Python-owned contract routes without replacing Go-owned definitions."""

from copy import deepcopy
from pathlib import Path

import yaml
from app.main import create_app

ROOT = Path(__file__).resolve().parents[1]
SPEC = ROOT / "docs/openapi.yaml"


def oas30(value):
    if isinstance(value, list):
        return [oas30(item) for item in value]
    if not isinstance(value, dict):
        return value
    result = {key: oas30(item) for key, item in value.items() if key not in {"const", "$schema"}}
    if "const" in value:
        result["enum"] = [value["const"]]
    variants = value.get("anyOf")
    if variants and any(item.get("type") == "null" for item in variants):
        nonnull = [oas30(item) for item in variants if item.get("type") != "null"]
        result.pop("anyOf", None)
        if len(nonnull) == 1:
            item = nonnull[0]
            result.update({"allOf": [item]} if "$ref" in item else item)
        else:
            result["anyOf"] = nonnull
        result["nullable"] = True
    if "HTTPBearer" in result:
        result["bearerAuth"] = result.pop("HTTPBearer")
    return result


def main():
    spec = yaml.safe_load(SPEC.read_text())
    runtime = oas30(create_app().openapi())
    for name, schema in runtime["components"]["schemas"].items():
        spec["components"]["schemas"][name] = schema
    spec["components"]["securitySchemes"]["serviceAuth"] = {
        "type": "http",
        "scheme": "bearer",
        "description": "Opaque internal service token. Server-to-server only.",
    }
    prefixes = (
        "/api/support",
        "/api/admin",
        "/api/notifications",
        "/api/event-images",
        "/internal/",
    )
    for path, item in runtime["paths"].items():
        if not path.startswith(prefixes) and path not in {
            "/api/auth/logout",
            "/api/auth/resend-verification",
        }:
            continue
        spec["paths"][path] = deepcopy(item)
        for method, operation in spec["paths"][path].items():
            if method not in {"get", "post", "patch", "put", "delete"}:
                continue
            operation["description"] = (
                operation.get("summary", "")
                + ". Python-owned implementation with live identity and scoped permissions."
            )
            operation["x-implementation-status"] = "implemented"
            if path.startswith("/api/event-images/"):
                if method == "get":
                    operation["responses"]["200"]["content"] = {
                        "image/webp": {"schema": {"type": "string", "format": "binary"}}
                    }
                if method == "put":
                    operation["requestBody"] = {
                        "required": True,
                        "content": {
                            mime: {"schema": {"type": "string", "format": "binary"}}
                            for mime in ["image/png", "image/jpeg", "image/webp"]
                        },
                    }
                operation["responses"]["415"] = {"$ref": "#/components/responses/APIError"}
            if path.startswith("/api/admin/business/"):
                operation["x-upstream-dependency"] = (
                    "Go administration API; unavailable upstream fails closed."
                )
            operation["security"] = [
                {"serviceAuth": []} if path.startswith("/internal/") else {"bearerAuth": []}
            ]
            if path == "/api/auth/resend-verification" or (
                path.startswith("/api/event-images/") and method == "get"
            ):
                operation.pop("security", None)
            for status in ["400", "401", "403", "404", "409", "413", "422", "429", "500", "503"]:
                operation["responses"][status] = {"$ref": "#/components/responses/APIError"}
            if (
                path.endswith("/messages")
                and method == "post"
                or path == "/api/support/cases"
                and method == "post"
            ):
                operation["responses"]["200"] = deepcopy(operation["responses"]["201"])
                operation["responses"]["200"]["description"] = (
                    "Replay of the same idempotent request"
                )
    for path in ["/api/auth/register", "/api/auth/login", "/api/auth/request-password-reset"]:
        spec["paths"][path]["post"]["responses"]["429"] = {
            "$ref": "#/components/responses/APIError"
        }
    spec["paths"]["/api/auth/register"]["post"]["description"] = (
        "Creates attendee and encrypted durable verification job atomically. "
        "SMTP failure leaves a visible retryable job, not a lost account. "
        "Delivery is asynchronous; normal response remains 201."
    )
    spec["info"]["version"] = "0.3.0"
    spec["info"]["description"] = (
        "Shared identity, support, notifications, administration and Go business contract. "
        "Implementation status is explicit; planned Go routes are not completed functionality."
    )
    spec["servers"][0]["description"] = (
        "Python identity/support/notifications/admin and private internal APIs"
    )

    def schema_refs(value):
        found = set()
        if isinstance(value, dict):
            ref = value.get("$ref", "")
            if ref.startswith("#/components/schemas/"):
                found.add(ref.rsplit("/", 1)[1])
            for child in value.values():
                found.update(schema_refs(child))
        elif isinstance(value, list):
            for child in value:
                found.update(schema_refs(child))
        return found

    needed = schema_refs(spec["paths"]) | schema_refs(spec["components"].get("responses", {}))
    while True:
        expanded = needed | set().union(
            *(schema_refs(spec["components"]["schemas"][name]) for name in needed)
        )
        if expanded == needed:
            break
        needed = expanded
    spec["components"]["schemas"] = {
        name: item for name, item in spec["components"]["schemas"].items() if name in needed
    }
    SPEC.write_text(yaml.safe_dump(spec, sort_keys=False, allow_unicode=True, width=100))


if __name__ == "__main__":
    main()
