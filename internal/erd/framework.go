package erd

import "sort"

// frameworkSet is the union of known ORM/framework infrastructure table names.
var frameworkSet = func() map[string]struct{} {
	names := []string{
		// Laravel
		"migrations", "cache", "cache_locks", "jobs", "job_batches", "failed_jobs",
		"sessions", "password_reset_tokens", "personal_access_tokens",
		// Rails
		"schema_migrations", "ar_internal_metadata", "active_storage_blobs",
		"active_storage_attachments", "active_storage_variant_records", "action_text_rich_texts",
		// Django
		"django_migrations", "django_content_type", "django_session", "django_admin_log",
		"auth_user", "auth_group", "auth_permission", "auth_group_permissions",
		"auth_user_groups", "auth_user_user_permissions",
		// Prisma
		"_prisma_migrations",
	}
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		m[n] = struct{}{}
	}
	return m
}()

// DetectFrameworkTables returns sorted table names from schema that match known
// ORM/framework infrastructure tables.
func DetectFrameworkTables(schema []Table) []string {
	var out []string
	for _, t := range schema {
		if _, ok := frameworkSet[t.Name]; ok {
			out = append(out, t.Name)
		}
	}
	sort.Strings(out)
	return out
}
