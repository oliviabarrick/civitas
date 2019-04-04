variable "digitalocean_token" {}

variable "num_nodes" {
  default = "4"
}

resource "digitalocean_droplet" "node" {
  count = "${var.num_nodes}"

  image  = "coreos-stable"
  name   = "civitas-${count.index}"
  region = "sfo2"
  size   = "s-2vcpu-2gb"
  private_networking = true

  ssh_keys = [
    "2b:07:2e:3e:13:13:c0:9a:c7:4f:71:0e:81:01:a4:4d",
    "c9:eb:65:63:44:bc:ca:85:50:c0:6f:88:6a:03:1e:55"
  ]

  lifecycle {
    ignore_changes = ["volume_ids"]
  }

  tags = ["civitas"]

  user_data = <<EOF
#cloud-config

coreos:
  units:
    - name: kubernetes.service
      enable: true
      command: start
      content: |
        [Unit]
        Description=Kubernetes
        After=docker.service

        [Service]
        Restart=always
        ExecStart=/usr/bin/docker run --name civitas --privileged --net=host --tmpfs /run --tmpfs /run/lock -v /sys/fs/cgroup:/sys/fs/cgroup:ro -e DISCOVERY_CONFIG="provider=digitalocean region=sfo2 tag_name=civitas api_token=${var.digitalocean_token}" -e ADVERTISE_INTERFACE=eth1 justinbarrick/civitas:dev
EOF
}

output "ips" {
  value = "${digitalocean_droplet.node.*.ipv4_address}"
}

provider "digitalocean" {
  token = "${var.digitalocean_token}"
}
