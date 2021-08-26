package onefss3

import (
	"context"
	"fmt"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	"time"
)

const (
	apiPathCreds                  string = "creds"
	defaultPathCredsRandomLength  int    = 6
	defaultPathCredsTimeFormat    string = "20060102150405"
	defaultPathCredsExpireSprintf string = "%s_%s_%s_%s"
	defaultPathCredsInfSprintf    string = "%s_%s_%s_INF_%s"
	fieldPathCredsName            string = "name"
	fieldPathCredsTTL             string = "ttl"
)

func pathCredsBuild(b *backend) []*framework.Path {
	return []*framework.Path{
		{
			Pattern: apiPathCreds + "/" + framework.GenericNameRegex(fieldPathCredsName),
			Fields: map[string]*framework.FieldSchema{
				fieldPathCredsName: {
					Type:        framework.TypeString,
					Description: "Name of the role to get an access token and secret",
				},
				fieldPathCredsTTL: {
					Type:        framework.TypeInt,
					Description: "Requested credentials duration in seconds. If not set or set to 0, configured default will be used. If set to -1, an unlimited duration credential will be requested if possible. Otherwise the maximum lease time will be granted.",
				},
			},
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.ReadOperation: &framework.PathOperation{Callback: b.pathCredsRead},
			},
		},
	}
}

// pathCredsRead
// Returns
// access_key is a text string of the access ID
// secret_key is a text string of the access ID secret
// key_expirey is the expiration time of the access ID and secret given in UNIX epoch timestamp seconds.
func (b *backend) pathCredsRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	roleName := data.Get(fieldPathCredsName).(string)
	if roleName == "" {
		return logical.ErrorResponse("Unable to parse role name"), nil
	}
	var credTTL int = 0
	TTLDuration, ok := data.GetOk(fieldPathCredsTTL)
	if ok {
		credTTL = TTLDuration.(int)
	}
	// Get configuration from backend storage
	role, err := getRoleFromStorage(ctx, req.Storage, roleName)
	if err != nil || role == nil {
		return nil, err
	}
	cfg, err := getCfgFromStorage(ctx, req.Storage)
	if err != nil || cfg == nil {
		return nil, err
	}
	// Calculate actual TTL in minutes based on the requested TTL and the rules in the role and plugin config
	maxTTL := CalcMaxTTL(role.TTLMax, cfg.TTLMax)
	TTLSeconds := CalcTTL(credTTL, role.TTL, cfg.TTL, maxTTL)
	TTLMinutes := 0
	if TTLSeconds > 0 {
		TTLMinutes = RoundTTLToUnit(TTLSeconds, TTLTimeUnit) / TTLTimeUnit
	} else {
		TTLMinutes = TTLSeconds // The TTL should be 0 or -1 which results in an infinite lease
	}

	// Generate username
	// If there is a TTL > 0 then the format of the user name has 4 parts:
	// Username prefix, random string, first 4 digits of Vault request UUID, and the expiration time
	// If the TTL is 0 or -1 (no TTL), the format has 5 parts:
	// Username prefix, random string, first 4 digits of Vault request UUID, the string INF, the create time for the user instead of expiration time
	randString, err := GenerateRandomString(defaultPathCredsRandomLength)
	if err != nil {
		return nil, err
	}
	credTime := time.Now().Local()
	credTimeString := defaultPathCredsInfSprintf
	if TTLMinutes > 0 {
		credTime = credTime.Add(time.Duration(TTLMinutes*TTLTimeUnit) * time.Second)
		credTimeString = defaultPathCredsExpireSprintf
	}
	username := fmt.Sprintf(credTimeString, cfg.UsernamePrefix, randString, req.ID[0:4], credTime.Format(defaultPathCredsTimeFormat))

	// Create the user
	_, err = b.Conn.PapiCreateUser(username, cfg.HomeDir, cfg.PrimaryGroup, role.AccessZone)
	if err != nil {
		return nil, fmt.Errorf("Error creating user: %s", err)
	}

	// Update user with all the appropriate group memberships from the role
	err = b.Conn.PapiSetUserSuplementalGroups(username, role.Groups, role.AccessZone)
	if err != nil {
		return nil, fmt.Errorf("Error setting user's supplemental groups: %s", err)
	}

	// Get the S3 access ID and secret key
	token, err := b.Conn.PapiGetS3Token(username, role.AccessZone, 0)
	if err != nil {
		return nil, fmt.Errorf("Unable to get S3 token for user %s: %s", username, err)
	}
	// Fill a key value struct with the stored values
	kv := map[string]interface{}{
		"access_key": token.AccessID,
		"secret_key": token.SecretKey,
		"key_expiry": 0, // 0 represents no expiration
	}
	// To have a token automatically expire, you need to create a second token and set the expiration duration of the previous token
	if TTLMinutes > 0 {
		token2, err := b.Conn.PapiGetS3Token(username, role.AccessZone, TTLMinutes)
		if err != nil {
			return nil, fmt.Errorf("Unable to get the second S3 token for user %s: %s", username, err)
		}
		kv["key_expiry"] = token2.OldKeyExpiry
	}

	res := &logical.Response{Data: kv}
	return res, nil
}
