// Build from meta-repo root: docker buildx bake [target]
// Example: docker buildx bake gateway

group "default" {
  targets = ["gateway", "auth", "users", "listings", "mentors", "sessions", "notifications", "payments", "metrics", "ui"]
}

target "_meta" {
  context = "."
}

target "gateway" {
  inherits = ["_meta"]
  dockerfile = "services/gateway/Dockerfile"
  tags = ["osship-gateway:local"]
}

target "auth" {
  inherits = ["_meta"]
  dockerfile = "services/auth/Dockerfile"
  tags = ["osship-auth:local"]
}

target "users" {
  inherits = ["_meta"]
  dockerfile = "services/users/Dockerfile"
  tags = ["osship-users:local"]
}

target "listings" {
  inherits = ["_meta"]
  dockerfile = "services/listings/Dockerfile"
  tags = ["osship-listings:local"]
}

target "mentors" {
  inherits = ["_meta"]
  dockerfile = "services/mentors/Dockerfile"
  tags = ["osship-mentors:local"]
}

target "sessions" {
  inherits = ["_meta"]
  dockerfile = "services/sessions/Dockerfile"
  tags = ["osship-sessions:local"]
}

target "notifications" {
  inherits = ["_meta"]
  dockerfile = "services/notifications/Dockerfile"
  tags = ["osship-notifications:local"]
}

target "payments" {
  context = "services/payments"
  dockerfile = "Dockerfile"
  tags = ["osship-payments:local"]
}

target "metrics" {
  context = "services/metrics"
  dockerfile = "Dockerfile"
  tags = ["osship-metrics:local"]
}

target "ui" {
  context = "ui"
  dockerfile = "Dockerfile"
  tags = ["osship-ui:local"]
}
