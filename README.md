# OneFS S3 secrets plug-in for Hashicorp Vault
Create dynamic secrets, access key ID and secret, for accessing OneFS S3 buckets.

## How it works
This plug-in dynamically creates local users on the OneFS cluster that belong to different groups depending on their roles. The S3 buckets created in OneFS are configured to have bucket ACLs providing access to the same groups that are configured for a user. This allows many users to access the same bucket without adding and removing user accounts to each bucket individually.

## Setup



## Plug-in configuration
To configure the plug-in you need to write a set of key/value pairs to the path /config/root off of your plug-in root path. The following table shows the required parameters and the optional ones.

| Key               | Description | Default | Required |
| ----------------- | ------------| :------ | :------: |
| endpoint          | **string** - FQDN or IP address of the OneFS cluster. The string should contain the protocol and port. e.g. https://cluster.name:8080 | | Yes |
| user              | **string** - User name for the user that will be used to access the OneFS cluster over the PAPI | | Yes |
| password          | **string** - Password for the user that will be used to access the OneFS cluster over the PAPI | | Yes |
| bypass_cert_check | **boolean** - When set to *true* SSL self-signed certificate issues are bypassed | false | No |
| cleanup_period    | **integer** - Number of seconds between calls to cleanup user accounts | 600 | No |
| homedir           | **string** - Path within /ifs where all user's home directories will point | /ifs/home/vault | No |
| primary_group     | **string** - Name of the primary group used by all users created by this plug-in. The group must already exist in any access zone on the cluster where S3 user accounts will be used | vault | No |
| ttl               | **int** - Default number of seconds that a secret token is valid. Individual roles and requests can override this value. A value of -1 or 0 represents an unlimited lifetime token. This value will be limited by the ttl_max value | 300 | No |
| ttl_max           | **int** - Maximum number of seconds a secret token can be valid. Individual roles can be less than or equal to this value. A value of -1 or 0 represents an unlimited lifetime token | 0 | No |
| username_prefix   | **string** - String to be used as the prefix for all users dynamically created by the plug-in | vault | No |

These configuration values should be written as key value pairs.

Example using the _vault_ command directly:

	vault write onefss3/config/root user="username" password="isasecret" endpoint="https://cluster.com:8080"

## General cluster configuration
* A local group needs to be configured in each access zone. This group is used as the primary group for all dynamically created users. The default group name assumed by the plug-in is __vault__. The group should have the same name in each access zone. It is not necessary for the groups to have the same GID.
* A common home directory under /ifs needs to be created. This directory will be the home directory for all dynamically created users in all access zones. The permission on the directory should be 755. The owner and group owner does not .
* A user account with the following privileges in the System zone is required for the plug-in to generate S3 secrets as well as add and delete users accounts as required. The name and password are required configuration parameters for the plug-in.
    * ISI_PRIV_AUTH with read and write permission
    * ISI_PRIV_LOGIN_PAPI with read permission
    * ISI_PRIV_S3 with read and write permission

## Example cluster configuration commands
### Create the local user, group, and default home directory
    isi auth users create vault_mgr --enabled=true --set-password
    isi auth groups create vault
    mkdir /ifs/home/vault
    chown root:wheel /ifs/home/vault

### Create a role that has the proper privileges and assign our new user to the role
    isi auth roles create --name=VaultMgr
    isi auth roles modify VaultMgr --add-user=vault_mgr
    isi auth roles modify VaultMgr --add-priv=ISI_PRIV_S3
    isi auth roles modify VaultMgr --add-priv=ISI_PRIV_AUTH
    isi auth roles modify VaultMgr --add-priv-ro=ISI_PRIV_LOGIN_PAPI

