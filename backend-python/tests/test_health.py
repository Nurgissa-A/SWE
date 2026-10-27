from fastapi.testclient import TestClient

from app.main import create_app


def test_liveness(settings):
    with TestClient(create_app(settings)) as client:
        response = client.get("/health")
        assert response.status_code == 200
        assert response.json() == {"status": "ok"}
        assert response.headers["X-Request-ID"]


def test_readiness_fails_without_database(settings):
    app = create_app(
        settings.model_copy(
            update={"database_url": "postgresql+psycopg://localhost:1/missing?connect_timeout=1"}
        )
    )
    with TestClient(app) as client:
        assert client.get("/ready").status_code == 503
