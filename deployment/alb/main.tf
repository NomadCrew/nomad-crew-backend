# Reserve a global static IP
resource "google_compute_global_address" "nomadcrew_lb_ip" {
  name         = "nomadcrew-lb-ip"
  address_type = "EXTERNAL"
}

# Serverless NEGs for Cloud Run services
resource "google_compute_region_network_endpoint_group" "preview_neg" {
  name                  = "preview-neg"
  region                = "us-east1"
  network_endpoint_type = "SERVERLESS"
  cloud_run {
    service = "preview-environment"
  }
}

resource "google_compute_region_network_endpoint_group" "backend_neg" {
  name                  = "backend-neg"
  region                = "us-east1"
  network_endpoint_type = "SERVERLESS"
  cloud_run {
    service = "nomadcrew-backend"
  }
}

# Backend Services
resource "google_compute_backend_service" "preview_backend" {
  name                  = "preview-backend"
  load_balancing_scheme = "EXTERNAL"
  protocol              = "HTTP"
  backend {
    group = google_compute_region_network_endpoint_group.preview_neg.id
  }
}

resource "google_compute_backend_service" "backend_backend" {
  name                  = "backend-backend"
  load_balancing_scheme = "EXTERNAL"
  protocol              = "HTTP"
  backend {
    group = google_compute_region_network_endpoint_group.backend_neg.id
  }
}

# URL Map with Host Rules
resource "google_compute_url_map" "nomadcrew_url_map" {
  name            = "nomadcrew-url-map"
  default_service = google_compute_backend_service.backend_backend.id

  host_rule {
    hosts        = ["preview.${var.domain}"]
    path_matcher = "preview-matcher"
  }

  path_matcher {
    name            = "preview-matcher"
    default_service = google_compute_backend_service.preview_backend.id
  }
}

# Google-Managed SSL Certificate
resource "google_compute_managed_ssl_certificate" "nomadcrew_ssl_cert" {
  name = "nomadcrew-ssl-cert"
  managed {
    domains = ["preview.${var.domain}", "api.${var.domain}"]
  }
}

# HTTPS Target Proxy
resource "google_compute_target_https_proxy" "nomadcrew_https_proxy" {
  name             = "nomadcrew-https-proxy"
  url_map          = google_compute_url_map.nomadcrew_url_map.id
  ssl_certificates = [google_compute_managed_ssl_certificate.nomadcrew_ssl_cert.id]
}

# Global Forwarding Rule
resource "google_compute_global_forwarding_rule" "nomadcrew_https_rule" {
  name                  = "nomadcrew-https-rule"
  target                = google_compute_target_https_proxy.nomadcrew_https_proxy.id
  ip_address            = google_compute_global_address.nomadcrew_lb_ip.address
  port_range            = "443"
  load_balancing_scheme = "EXTERNAL"
}

# Optional: HTTP redirect to HTTPS
resource "google_compute_url_map" "http_redirect" {
  name = "nomadcrew-http-redirect"
  default_url_redirect {
    https_redirect = true
    strip_query    = false
  }
}

resource "google_compute_target_http_proxy" "nomadcrew_http_proxy" {
  name    = "nomadcrew-http-proxy"
  url_map = google_compute_url_map.http_redirect.id
}

resource "google_compute_global_forwarding_rule" "nomadcrew_http_rule" {
  name                  = "nomadcrew-http-rule"
  target                = google_compute_target_http_proxy.nomadcrew_http_proxy.id
  ip_address            = google_compute_global_address.nomadcrew_lb_ip.address
  port_range            = "80"
  load_balancing_scheme = "EXTERNAL"
}

resource "google_cloud_run_service_iam_member" "preview_noauth" {
  location = google_compute_region_network_endpoint_group.preview_neg.region
  service  = "preview-environment"
  role     = "roles/run.invoker"
  member   = "allUsers"
}