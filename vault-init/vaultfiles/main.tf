provider "aws" {
  region                  = "us-west-2"
  shared_credentials_file = "~/.aws/credentials"
  profile                 = "default"
}

resource "aws_instance" "web" {
    ami = "${var.ami}"
    instance_type = "${var.instance_type}"
    key_name = "${var.key-name}"
    security_groups = ["access-https","vault-ssh", "vault-egress"]
    tags{
        Name = "vault"
    }
    provisioner "file" {
        source      = "../../vault_properties.hcl"
        destination = "/tmp/vault_properties.hcl"
        connection {
            private_key = "${file("${var.deploy-pem-path}")}"
            user = "ubuntu"
            //agent = true
        }
    }
    provisioner "file" {
        connection {
            private_key = "${file("${var.deploy-pem-path}")}"
            user="ubuntu"
            //agent = true
        }
        source      = "../../certs/cert_files/serv_cert.pem"
        destination = "/tmp/serv_cert.pem"
    }

    provisioner "file" {
        connection {
            private_key = "${file("${var.deploy-pem-path}")}"
            user="ubuntu"
            //agent = true
        }
        source      = "../../certs/cert_files/serv_key.pem"
        destination = "/tmp/serv_key.pem"
    }

    provisioner "file" {
        connection {
            private_key = "${file("${var.deploy-pem-path}")}"
            user="ubuntu"
            //agent = true
        }
        source      = "${path.module}/scripts/install.sh"
        destination = "/tmp/install.sh"
    }

   provisioner "remote-exec" {
       inline = [
       "chmod +x /tmp/install.sh",
       "/tmp/install.sh"
        ]
        connection {
            type        = "ssh"
            agent       = false
            user        = "ubuntu"
            private_key = "${file("${var.deploy-pem-path}")}"
        }
    }
}

// This rule allows Vault HTTPS API access to individual nodes, since each will
// need to be addressed individually for unsealing.
resource "aws_security_group" "access-https" {
  name = "access-https"
  ingress {
    from_port   = 8200
    to_port     = 8200
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
//generic security groups

resource "aws_security_group" "vault-ssh" {
  name = "vault-ssh"
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "vault-egress" {
    name = "vault-egress"
    egress {
        from_port   = 0
        to_port     = 0
        protocol    = "-1"
        cidr_blocks = ["0.0.0.0/0"]
    }
}

