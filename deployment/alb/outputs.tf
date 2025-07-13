output "load_balancer_ip" {
  value       = google_compute_global_address.nomadcrew_lb_ip.address
  description = "Public IP address of the load balancer"
}