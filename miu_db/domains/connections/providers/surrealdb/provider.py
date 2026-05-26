"""Provider registration for SurrealDB."""

from miu_db.domains.connections.providers.adapter_provider import build_adapter_provider
from miu_db.domains.connections.providers.catalog import register_provider
from miu_db.domains.connections.providers.model import DatabaseProvider, ProviderSpec
from miu_db.domains.connections.providers.surrealdb.schema import SCHEMA


def _provider_factory(spec: ProviderSpec) -> DatabaseProvider:
    from miu_db.domains.connections.providers.surrealdb.adapter import SurrealDBAdapter

    return build_adapter_provider(spec, SCHEMA, SurrealDBAdapter())


SPEC = ProviderSpec(
    db_type="surrealdb",
    display_name="SurrealDB",
    schema_path=("miu_db.domains.connections.providers.surrealdb.schema", "SCHEMA"),
    supports_ssh=True,
    is_file_based=False,
    has_advanced_auth=False,
    default_port="8000",
    requires_auth=True,
    badge_label="Surreal",
    url_schemes=("surrealdb", "surreal"),
    provider_factory=_provider_factory,
)

register_provider(SPEC)
