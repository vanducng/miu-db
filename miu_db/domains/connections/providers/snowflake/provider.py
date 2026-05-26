"""Provider registration."""

from miu_db.domains.connections.providers.adapter_provider import build_adapter_provider
from miu_db.domains.connections.providers.catalog import register_provider
from miu_db.domains.connections.providers.model import DatabaseProvider, ProviderSpec
from miu_db.domains.connections.providers.snowflake.schema import SCHEMA


def _provider_factory(spec: ProviderSpec) -> DatabaseProvider:
    from miu_db.domains.connections.providers.snowflake.adapter import SnowflakeAdapter

    return build_adapter_provider(spec, SCHEMA, SnowflakeAdapter())

SPEC = ProviderSpec(
    db_type="snowflake",
    display_name="Snowflake",
    schema_path=("miu_db.domains.connections.providers.snowflake.schema", "SCHEMA"),
    supports_ssh=False,
    is_file_based=False,
    has_advanced_auth=False,
    default_port="",
    requires_auth=True,
    badge_label="SNOW",
    provider_factory=_provider_factory,
)

register_provider(SPEC)
