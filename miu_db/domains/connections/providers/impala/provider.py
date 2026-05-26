"""Provider registration for Impala."""

from miu_db.domains.connections.providers.adapter_provider import build_adapter_provider
from miu_db.domains.connections.providers.catalog import register_provider
from miu_db.domains.connections.providers.impala.schema import SCHEMA
from miu_db.domains.connections.providers.model import DatabaseProvider, ProviderSpec


def _provider_factory(spec: ProviderSpec) -> DatabaseProvider:
    from miu_db.domains.connections.providers.impala.adapter import ImpalaAdapter

    return build_adapter_provider(spec, SCHEMA, ImpalaAdapter())


SPEC = ProviderSpec(
    db_type="impala",
    display_name="Impala",
    schema_path=("miu_db.domains.connections.providers.impala.schema", "SCHEMA"),
    supports_ssh=True,
    is_file_based=False,
    has_advanced_auth=True,  # Kerberos support
    default_port="21050",
    requires_auth=False,
    badge_label="Impala",
    url_schemes=("impala",),
    provider_factory=_provider_factory,
)

register_provider(SPEC)
