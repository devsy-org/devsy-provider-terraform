variable "disk_image" {
  type = string
  default = "ami-0c1bea58988a989155"
}

variable "state" {
  type = string
  default = "running"
}

variable "machine_name" {
  type = string
  default = "devsy-devsy"
}

variable "disk_size" {
  type = string
  default = "50"
}

variable "instance_type" {
  type = string
  default = "t2.micro"
}

variable "vpc" {
  type = string
  default = ""
}

variable "region" {
  type = string
  default = ""
}

variable "ssh_key" {
  type = string
  default = "invalid"
}

provider "aws" {
  region = "${var.region}"
}


resource "aws_security_group" "devsy" {
  name = "devsy"
  description = "devsy"
  vpc_id = "${var.vpc}"

  // To Allow SSH Transport
  ingress {
    from_port = 22
    protocol = "tcp"
    to_port = 22
    cidr_blocks = ["0.0.0.0/0"]
  }

  lifecycle {
    create_before_destroy = true
  }
}


resource "aws_instance" "devsy" {
  ami = "${var.disk_image}"
  instance_type = "${var.instance_type}"

  vpc_security_group_ids = [
    aws_security_group.devsy.id
  ]
  root_block_device {
    delete_on_termination = true
    volume_size = "${var.disk_size}"
  }
  tags = {
    Name = "devsy"
    devsy = "${var.machine_name}"
  }

  user_data = <<EOF
#!/bin/sh
useradd devsy -d /home/devsy
mkdir -p /home/devsy
if grep -q sudo /etc/groups; then
  usermod -aG sudo devsy
elif grep -q wheel /etc/groups; then
  usermod -aG wheel devsy
fi
echo "devsy ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/91-devsy
mkdir -p /home/devsy/.ssh
echo "${var.ssh_key}" >> /home/devsy/.ssh/authorized_keys
chmod 0700 /home/devsy/.ssh
chmod 0600 /home/devsy/.ssh/authorized_keys
chown -R devsy:devsy /home/devsy
echo "${var.state}"
EOF

  depends_on = [ aws_security_group.devsy ]
}

output "public_ip" {
  value = aws_instance.devsy.public_ip
}
