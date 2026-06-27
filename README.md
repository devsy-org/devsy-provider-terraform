# Terraform Provider for Devsy

## Getting started

The provider is available for auto-installation using

```sh
devsy provider add terraform
devsy provider use terraform
```

Follow the on-screen instructions to complete the setup.

Needed variables will be:

- TERRAFORM_PROJECT
- REGION

`TERRAFORM_PROJECT` points to a git repo or directory where the terraform project
that defines the infra is stored.

In this repo it would point to: `./examples/terraform-aws/`

### Creating your first Devsy environment with terraform

After the initial setup, just use:

```sh
devsy workspace up .
```

You'll need to wait for the machine and environment setup.

### Notes

With the terraform provider, all the power is in the terraform project. So it
will be there where you will place your defaults for

- DISK_SIZE
- IMAGE_DISK
- INSTANCE_TYPE

Keep also in mind that **stop/start is not supported right now on the terraform provider**.
So the right thing to do is to handle data saving inside your terraform code
(e.g. use external data buckets).

### Customize the VM Instance

This provider has the following options:

| NAME              | REQUIRED | DESCRIPTION                                                                 | DEFAULT |
|-------------------|----------|-----------------------------------------------------------------------------|---------|
| TERRAFORM_PROJECT | true     | The path or repo where the terraform files are. E.g. ./examples/terraform   |         |
| REGION            | true     | The cloud region to create the VM in. E.g. us-west-1                        |         |
| DISK_SIZE         | false    | The disk size to use (GB).                                                  | 40      |
| IMAGE_DISK        | false    | The disk image to use.                                                      |         |
| INSTANCE_TYPE     | false    | The machine type to use.                                                    |         |

Options can either be set in `env` or using for example:

```sh
devsy provider set-options -o IMAGE_DISK=my-custom-ami
devsy provider set-options -o INSTANCE_TYPE=t2.micro
devsy provider set-options -o REGION=us-west-2
```

## Extra

For more detail, see the [Devsy Documentation](https://devsy.sh/docs/managing-providers/what-are-providers).
