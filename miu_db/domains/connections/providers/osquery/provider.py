"""Provider registration for osquery."""

from miu_db.domains.connections.providers.adapter_provider import build_adapter_provider
from miu_db.domains.connections.providers.catalog import register_provider
from miu_db.domains.connections.providers.model import DatabaseProvider, ProviderSpec
from miu_db.domains.connections.providers.osquery.schema import SCHEMA


def _provider_factory(spec: ProviderSpec) -> DatabaseProvider:
    from miu_db.domains.connections.providers.osquery.adapter import OsqueryAdapter

    return build_adapter_provider(spec, SCHEMA, OsqueryAdapter())


SPEC = ProviderSpec(
    db_type="osquery",
    display_name="osquery",
    schema_path=("miu_db.domains.connections.providers.osquery.schema", "SCHEMA"),
    supports_ssh=False,
    is_file_based=False,
    has_advanced_auth=False,
    default_port="",
    requires_auth=False,
    badge_label="osq",
    url_schemes=("osquery",),
    provider_factory=_provider_factory,
)

register_provider(SPEC)
