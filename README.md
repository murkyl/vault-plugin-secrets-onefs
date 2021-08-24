# OneFS S3 secrets plugin for Hashicorp Vault
Create dynamic secrets, access key ID and secret, for accessing OneFS S3 buckets.

## How it works
This plugin dynamically creates local users on the OneFS cluster that belong to different groups depending on their roles. The S3 buckets created in OneFS are configured to have bucket ACLs providing access to the same groups that are configured for a user. This allows many users to access the same bucket without adding and removing user accounts to each bucket individually.

## Installation
A secrets engine plugin must be setup and configured before it can be used. Follow the directions below to properly install and configure the plugin.

### Using pre-built releases (recommended)
Any binary releases available can be found [here](https://github.com/murkyl/vault-plugin-secrets-onefss3/releases).

### From source
Clone the GitHub repository to your local machine and run `make build` from the root of the sources diretory. After successful compilation the resulting `vault-plugin-secrets-onefss3` binary is located in the `bin/` directory.

Building from source assumes you have installed the Go development environment.

### Registering the plugin
Before a Vault plugin can be used, it must be copied to the Vault plugins directory and registered. The plugin directory for Vault is located in the Vault configuration file, often located at `/etc/vault.d/vault.hcl`.

Details of the Vault configuration file can be found in the [Vault online docs](https://www.vaultproject.io/docs/configuration "docs").

The required settings for registering a plugin are `plugin_directory` and `api_addr`. These need to be set according to your environment.

After copying the binary into the plugin directory, make sure that the permissions on the binary allow the Vault server process to execute it. Sometimes this means doing a `chown vault:vault` and a `chmod 750`.

Make sure the Vault server itself is running, unsealed, and your have logged into Vault before registering the plugin.

Plugins also need to be registered with the Vault server [plugin catalog](https://www.vaultproject.io/docs/internals/plugins.html#plugin-catalog) before they can be enabled. To do this the SHA256 sum of the binary needs to be obtained and used in the register command.
```shell
vault plugin register \
	-sha256=$(sha256sum /etc/vault.d/vault_plugins/vault-plugin-secrets-onefs-s3 | cut -d " " -f 1) \
	secret vault-plugin-secrets-onefs-s3
```

### Enabling the plugin
After the plugin is registered you can enable the plugin and have it available on a mount path.
```shell
vault secrets enable -path=onefss3 vault-plugin-secrets-onefs-s3
```

### Plugin configuration
To configure the plugin you need to write a set of key/value pairs to the path /config/root off of your plugin mount point. These configuration values should be written as key value pairs. Only 3 values are mandatory while the remainder have defaults. See the [available options](#path-configroot) below for additional customization.

```shell
vault write onefss3/config/root \
    user="username" \
    password="isasecret" \
    endpoint="https://cluster.com:8080"
```

### General OneFS cluster configuration
* A local group needs to be configured in each access zone. This group is used as the primary group for all dynamically created users. The default group name assumed by the plugin is __vault__. The group should have the same name in each access zone. It is not necessary for the groups to have the same GID.
* A common home directory under /ifs needs to be created. This directory will be the home directory for all dynamically created users in all access zones. The permission on the directory should be 755. The owner and group owner does not .
* A user account with the following privileges in the System zone is required for the plugin to generate S3 secrets as well as add and delete users accounts as required. The name and password are required configuration parameters for the plugin.
    * ISI_PRIV_AUTH with read and write permission
    * ISI_PRIV_LOGIN_PAPI with read permission
    * ISI_PRIV_S3 with read and write permission

An example of the commands required on the OneFS cluster side follow.
#### Create the local user, group, and default home directory
    isi auth users create vault_mgr --enabled=true --set-password
    isi auth groups create vault
    mkdir /ifs/home/vault
    chown root:wheel /ifs/home/vault

#### Create a role that has the proper privileges and assign our new user to the role
    isi auth roles create --name=VaultMgr
    isi auth roles modify VaultMgr --add-user=vault_mgr
    isi auth roles modify VaultMgr --add-priv=ISI_PRIV_S3
    isi auth roles modify VaultMgr --add-priv=ISI_PRIV_AUTH
    isi auth roles modify VaultMgr --add-priv-ro=ISI_PRIV_LOGIN_PAPI

## Usage
Normal use involves creating roles that associate local groups to the role and then retrieving the credentials for that role. The roles and credential paths need to be secured via ACLs in Vault itself as the plugin does not perform any authentication or access control. Any request that reaches the plugin is assumed ot have permission to do so from Vault.

#### Create an initial test S3 bucket on the OneFS cluster and add ACEs
    isi s3 buckets create --create-path --name=testbucket --path=/ifs/testbucket --owner=root
    isi s3 buckets modify testbucket --add-ace="name=Backup Operators,type=group,perm=full_control"
    isi s3 buckets modify testbucket --add-ace="name=Guests,type=group,perm=full_control"

### Create a plugin role in Vault that will have access to the test bucket
This plugin role will associate local groups to the dynamic user and since the bucket ACL has these groups the dynamic user will have access to the bucket. Backup Operators and Guests are used in this example as they are available by default on the cluster.
```shell
vault write onefss3/roles/Test1 group="Guests" group="Backup Operators" bucket="s3-test" access_zone="System"
```
The access zone is required when defining a role. See the [available options](#path-rolesrole_name) below for additional customization.

### Retrieve a credential
```shell
vault read onefss3/creds/Test1
```

### Retrieve a credential with a specific TTL
```shell
vault read onefss3/creds/Test1 ttl=-1
vault read onefss3/creds/Test1 ttl=180
```

## Plugin options
### Available paths
    /config/root
    /roles/_role_name_
    /creds/_role_name_

### Available options
The configured TTL values for the role and plugin itself can be any value however, all TTL value will get rounded to the nearest 60 seconds (1 minute) when actually used.

#### Path: /config/root
| Key               | Description | Default | Required |
| ----------------- | ------------| :------ | :------: |
| endpoint          | **string** - FQDN or IP address of the OneFS cluster. The string should contain the protocol and port. e.g. https://cluster.name:8080 | | Yes |
| user              | **string** - User name for the user that will be used to access the OneFS cluster over the PAPI | | Yes |
| password          | **string** - Password for the user that will be used to access the OneFS cluster over the PAPI | | Yes |
| bypass_cert_check | **boolean** - When set to *true* SSL self-signed certificate issues are bypassed | false | No |
| cleanup_period    | **integer** - Number of seconds between calls to cleanup user accounts | 600 | No |
| homedir           | **string** - Path within /ifs where all user's home directories will point | /ifs/home/vault | No |
| primary_group     | **string** - Name of the primary group used by all users created by this plugin. The group must already exist in any access zone on the cluster where S3 user accounts will be used | vault | No |
| ttl               | **int** - Default number of seconds that a secret token is valid. Individual roles and requests can override this value. A value of -1 or 0 represents an unlimited lifetime token. This value will be limited by the ttl_max value | 300 | No |
| ttl_max           | **int** - Maximum number of seconds a secret token can be valid. Individual roles can be less than or equal to this value. A value of -1 or 0 represents an unlimited lifetime token | 0 | No |
| username_prefix   | **string** - String to be used as the prefix for all users dynamically created by the plugin | vault | No |

#### Path: /roles/role_name
| Key               | Description | Default | Required |
| ----------------- | ------------| :------ | :------: |
| bucket            | **string** - Name of the S3 bucket | | Yes |
| group             | **string** - Name of the group(s) that this role will have. Use multiple group key/value pairs to specify multiple groups | | Yes |
| access_zone       | **string** - Access zone on the OneFS cluster that the role belongs | System | No |
| ttl               | **int** - Default number of seconds that a secret token is valid. Individual requests can override this value. A value of -1 or 0 represents an unlimited lifetime token. This value will be limited by the ttl_max value | -1 | No |
| ttl_max           | **int** - Maximum number of seconds a secret token can be valid. This value may be limited by plugin configuration. A value of -1 or 0 represents an unlimited lifetime token | -1 | No |

#### Path: /creds/role_name
| Key               | Description | Default | Required |
| ----------------- | ------------| :------ | :------: |
| ttl               | **int** - Requested number of seconds that  secret token is valid. This value will be capped by the maximum TTL specified by the role and plugin configuration. A value of -1 represents an unlimited lifetime token. A value of 0 represents taking the role or plugin configuration default | 0 | No |
