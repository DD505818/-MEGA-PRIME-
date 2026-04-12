import os

import jwt


INTER_SERVICE_SECRET = os.getenv("INTER_SERVICE_SECRET", "change-me-in-prod")


def sign_message(payload: dict) -> str:
    return jwt.encode(payload, INTER_SERVICE_SECRET, algorithm="HS256")


def verify_message(token: str, expected_payload: dict) -> bool:
    try:
        decoded = jwt.decode(token, INTER_SERVICE_SECRET, algorithms=["HS256"])
        return decoded == expected_payload
    except jwt.InvalidTokenError:
        return False
