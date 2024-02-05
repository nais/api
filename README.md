# NAIS API

## Local development

```bash
asdf install # Install required dependencies
docker compose up -d
make setup-local # Setup local environment (seeds database and creates pubsub topic)
make local
```

For local development you need to set the `WITH_FAKE_CLIENTS` environment variable to `true` (as set by `make local`),
and you will also need to set the `X-User-Email` header to `dev.usersen@example.com` if you want to act as a regular
user, or `admin.usersen@example.com` if you need an admin user.