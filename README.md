### OneFS S3 secrets plugin for Hashicorp Vault

# Plugin configuration
To configure the plugin you need to write a set of key/value pairs to the path /config/root off of your plugin root path. The following table shows the required parameters and the optional ones.

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

# Cluster configuration
* A local group needs to be configured in each access zone. This group is used as the primary group for all dynamically created users. The default group name assumed by the plugin is __vault__. The group should have the same name in each access zone. It is not necessary for the groups to have the same GID.
* Common home directory under /ifs needs to be created. This directory will be the home diretory for all dynamically created users in all access zones. The permission on the directory should be 755 and the owner should be root:wheel.
* A user account with the following privileges in the System zone is required for the plugin to generate S3 secrets as well as add and delete users accounts as required. The name and password are required configuration parameters for the plugin.
    * ISI_PRIV_AUTH
    * ISI_PRIV_LOGIN_PAPI
    * ISI_PRIV_S3
