"""
Purpose: Locust scenarios for PRR-style traffic generation.
The flows focus on the same HTTP entry points used by the demo UI.
"""

import os
import uuid
from locust import HttpUser, task, between


class BaseUser(HttpUser):
    abstract = True
    wait_time = between(1, 3)
    host = os.getenv("TARGET_URL", "http://localhost:8080")

    def on_start(self):
        # Each virtual user gets a unique auth identity so the demo backend can issue a JWT reliably.
        self.username = f"locust-{uuid.uuid4().hex[:10]}"
        password = "pass"
        self.authenticated = False
        register = self.client.post(
            "/auth/register",
            json={"username": self.username, "password": password, "role": "user"},
            name="/auth/register",
        )
        if register.status_code not in (200, 201, 409):
            return

        response = self.client.post(
            "/auth/login",
            json={"username": self.username, "password": password},
            name="/auth/login",
        )
        if response.status_code != 200:
            return
        token = response.json()["token"]
        if not token:
            return
        self.client.headers.update({"Authorization": f"Bearer {token}"})
        self.authenticated = True


class BrowsingUser(BaseUser):
    @task(3)
    def inventory_read_paths(self):
        if not getattr(self, "authenticated", False):
            return
        self.client.get("/inventory/low", name="/inventory/low")
        self.client.get("/inventory/sku?sku=SKU-001", name="/inventory/sku")
        self.client.get("/inventory/warehouses", name="/inventory/warehouses")

    @task(2)
    def catalog_search(self):
        if not getattr(self, "authenticated", False):
            return
        self.client.get("/catalog/search?q=SKU", name="/catalog/search")

    @task(1)
    def order_lookup(self):
        if not getattr(self, "authenticated", False):
            return
        self.client.get("/orders", name="/orders[list]")


class OrderFlowUser(BaseUser):
    def on_start(self):
        super().on_start()
        self.order_id = None
        self.warehouse_id = None
        if not getattr(self, "authenticated", False):
            return
        warehouses = self.client.get("/inventory/warehouses", name="/inventory/warehouses")
        warehouses.raise_for_status()
        body = warehouses.json()
        items = body.get("warehouses") if isinstance(body, dict) else []
        if items:
            first = items[0]
            if isinstance(first, dict):
                self.warehouse_id = first.get("warehouse_id") or first.get("id")

    @task(2)
    def create_empty_order(self):
        if not getattr(self, "authenticated", False):
            return
        response = self.client.post(
            "/orders",
            json={"user_id": self.username},
            name="/orders[create]",
        )
        if response.status_code != 200:
            return
        self.order_id = response.json()["order_id"]

    @task(4)
    def reserve_sku(self):
        if not getattr(self, "authenticated", False) or not self.order_id:
            return
        if not self.warehouse_id:
            return
        payload = {
            "order_id": self.order_id,
            "items": [{"sku": "SKU-001", "warehouse_id": self.warehouse_id, "qty": 1}],
        }
        response = self.client.post("/inventory/reserve", json=payload, name="/inventory/reserve")
        if response.status_code != 200:
            return

    @task(2)
    def release_sku(self):
        if not getattr(self, "authenticated", False) or not self.order_id:
            return
        response = self.client.post(
            "/inventory/release",
            json={"order_id": self.order_id},
            name="/inventory/release",
        )
        if response.status_code != 200:
            return

    @task(1)
    def confirm_sku(self):
        if not getattr(self, "authenticated", False) or not self.order_id:
            return
        response = self.client.post(
            "/inventory/confirm",
            json={"order_id": self.order_id},
            name="/inventory/confirm",
        )
        if response.status_code != 200:
            return
