package vaultonefs

// TODO: Add code to check a bucket and make sure the groups added for a role exist on the bucket ACL list

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	"strings"
)

const (
	apiPathRolesDynamic                  string = "roles/dynamic/"
	apiPathRolesDynamicDefaultAccessZone string = "System"
	fieldPathRolesDynamicAccessZone      string = "access_zone"
	fieldPathRolesDynamicBucket          string = "bucket"
	fieldPathRolesDynamicGroup           string = "group"
	fieldPathRolesDynamicName            string = "name"
	fieldPathRolesDynamicTTL             string = "ttl"
	fieldPathRolesDynamicTTLMax          string = "ttl_max"
)

type s3Role struct {
	Bucket     string
	Groups     []string
	AccessZone string
	TTL        int
	TTLMax     int
}

func pathRolesDynamicBuild(b *backend) []*framework.Path {
	return []*framework.Path{
		{
			Pattern: apiPathRolesDynamic + framework.GenericNameRegex(fieldPathRolesDynamicName),
			Fields: map[string]*framework.FieldSchema{
				fieldPathRolesDynamicAccessZone: {
					Type:        framework.TypeString,
					Description: "Access zone that this role will apply.",
				},
				fieldPathRolesDynamicBucket: {
					Type:        framework.TypeString,
					Description: "Name of the bucket in the given access zone to associate this role against.",
				},
				fieldPathRolesDynamicGroup: {
					Type:        framework.TypeStringSlice,
					Description: "Name of group(s) that this role should belong. To specify multiple groups repeat the group=<group_name> key value pair. The groups specified here should already be in the ACL of the bucket.",
				},
				fieldPathRolesDynamicName: {
					Type:        framework.TypeString,
					Description: "Name of the role. The name should start and end with alphanumeric characters. Characters in the middle can be alphanumeric, . (period), or - (dash).",
				},
				fieldPathRolesDynamicTTL: {
					Type:        framework.TypeInt,
					Description: "Default credential duration in seconds. If not set or 0, plugin configuration will be used. If set to -1 no TTL will be used up to the plugin configuration.",
				},
				fieldPathRolesDynamicTTLMax: {
					Type:        framework.TypeInt,
					Description: "Maximum credential duration in seconds. If not set or 0, plugin configuration will be used. If set to -1, no TTL will be used up to the plugin configuration.",
				},
			},
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.CreateOperation: &framework.PathOperation{Callback: b.pathRolesDynamicWrite},
				logical.ReadOperation:   &framework.PathOperation{Callback: b.pathRolesDynamicRead},
				logical.UpdateOperation: &framework.PathOperation{Callback: b.pathRolesDynamicWrite},
				logical.DeleteOperation: &framework.PathOperation{Callback: b.pathRolesDynamicDelete},
			},
		},
	}
}

func (b *backend) pathRolesDynamicWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	roleName := data.Get(fieldPathRolesDynamicName).(string)
	if roleName == "" {
		return logical.ErrorResponse("Role name is missing"), nil
	}
	// Get existing role object or create a new one as necessary
	role, err := getDynamicRoleFromStorage(ctx, req.Storage, roleName)
	if err != nil {
		return nil, err
	}
	if role == nil {
		role = &s3Role{}
	}
	// Set role struct to values from request
	groupNames, ok := data.GetOk(fieldPathRolesDynamicGroup)
	if ok {
		role.Groups = groupNames.([]string)
	}
	bucketName, ok := data.GetOk(fieldPathRolesDynamicBucket)
	if ok {
		role.Bucket = bucketName.(string)
	}
	azName, ok := data.GetOk(fieldPathRolesDynamicAccessZone)
	if ok {
		role.AccessZone = azName.(string)
	}
	TTLDuration, ok := data.GetOk(fieldPathRolesDynamicTTL)
	if ok {
		role.TTL = TTLDuration.(int)
	}
	TTLMaxDuration, ok := data.GetOk(fieldPathRolesDynamicTTLMax)
	if ok {
		role.TTLMax = TTLMaxDuration.(int)
	}
	// Validate values
	var validationErrors []string
	if role.AccessZone == "" {
		role.AccessZone = apiPathRolesDynamicDefaultAccessZone
	}
	if role.Bucket == "" {
		validationErrors = append(validationErrors, "A bucket name is required for a role")
	}
	if role.Groups == nil {
		validationErrors = append(validationErrors, "A group of list of groups is required for a role")
	}
	if role.TTLMax < 0 {
		role.TTLMax = -1
	}
	if role.TTL < 0 {
		role.TTL = -1
	}

	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("Validation errors for role: %s\n%s", roleName, strings.Join(validationErrors[:], "\n"))
	}
	// Format and store data on the backend server
	entry, err := logical.StorageEntryJSON((apiPathRolesDynamic + roleName), role)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("Unable to create storage object for role: %s", roleName)
	}
	if err = req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}
	return nil, nil
}

func (b *backend) pathRolesDynamicRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	roleName := data.Get(fieldPathRolesDynamicName).(string)
	if roleName == "" {
		return logical.ErrorResponse("Unable to parse role name"), nil
	}
	role, err := getDynamicRoleFromStorage(ctx, req.Storage, roleName)
	if err != nil || role == nil {
		return nil, err
	}
	// Fill a key value struct with the stored values
	kv := map[string]interface{}{
		fieldPathRolesDynamicAccessZone: role.AccessZone,
		fieldPathRolesDynamicBucket:     role.Bucket,
		fieldPathRolesDynamicGroup:      role.Groups,
		fieldPathRolesDynamicTTL:        role.TTL,
		fieldPathRolesDynamicTTLMax:     role.TTLMax,
	}
	return &logical.Response{Data: kv}, nil
}

// pathRolesDynamicDelete removes a role from the system
func (b *backend) pathRolesDynamicDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	roleName := data.Get(fieldPathRolesDynamicName).(string)
	if roleName == "" {
		return logical.ErrorResponse("Unable to parse role name"), nil
	}
	if err := req.Storage.Delete(ctx, apiPathRolesDynamic+roleName); err != nil {
		return nil, err
	}
	return nil, nil
}

// getDynamicRoleFromStorage retrieves a roles configuration from the API backend server and returns it in a s3Role struct
func getDynamicRoleFromStorage(ctx context.Context, s logical.Storage, roleName string) (*s3Role, error) {
	data, err := s.Get(ctx, apiPathRolesDynamic+roleName)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	role := &s3Role{}
	if err := json.Unmarshal(data.Value, role); err != nil {
		return nil, err
	}
	return role, nil
}
