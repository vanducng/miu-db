"""Provider registration."""

from miu_db.domains.connections.providers.adapter_provider import build_adapter_provider
from miu_db.domains.connections.providers.athena.schema import SCHEMA
from miu_db.domains.connections.providers.catalog import register_provider
from miu_db.domains.connections.providers.model import DatabaseProvider, ProviderSpec


def _provider_factory(spec: ProviderSpec) -> DatabaseProvider:
    from miu_db.domains.connections.providers.athena.adapter import AthenaAdapter

    return build_adapter_provider(spec, SCHEMA, AthenaAdapter())

SPEC = ProviderSpec(
    db_type="athena",
    display_name="AWS Athena",
    schema_path=("miu_db.domains.connections.providers.athena.schema", "SCHEMA"),
    supports_ssh=False,
    is_file_based=False,
    has_advanced_auth=True,
    default_port="",
    requires_auth=True,
    badge_label="Athena",
    provider_factory=_provider_factory,
)

register_provider(SPEC)
