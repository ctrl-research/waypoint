-- +goose Up
-- Generic OIDC sign-in (Authentik, Keycloak, …): the provider's stable
-- subject identifier, alongside google_sub since one instance may offer
-- both providers.
ALTER TABLE users ADD COLUMN oidc_sub text UNIQUE;

-- An account must have at least one credential; oidc_sub now counts.
ALTER TABLE users DROP CONSTRAINT users_has_credential;
ALTER TABLE users ADD CONSTRAINT users_has_credential
    CHECK (google_sub IS NOT NULL OR oidc_sub IS NOT NULL OR password_hash IS NOT NULL);

-- +goose Down
ALTER TABLE users DROP CONSTRAINT users_has_credential;
ALTER TABLE users ADD CONSTRAINT users_has_credential
    CHECK (google_sub IS NOT NULL OR password_hash IS NOT NULL);
ALTER TABLE users DROP COLUMN oidc_sub;
