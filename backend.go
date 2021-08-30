package vaultonefs

import (
	"context"
	"fmt"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	papi "github.com/murkyl/go-papi-lite"
	"regexp"
	"strings"
	"time"
)

const backendHelp = `
The OneFS secrets plugin for Vault allows dynamic creation and removal of S3 access tokens and secrets.
The plugin supports creation of role based access controls through integration with on cluster configuration.
`
const defaultUserRegexp string = "^%s_[^_]+_[^_]+_(?P<TimeStamp>[0-9]{14})$"

type backend struct {
	*framework.Backend
	Conn        *papi.OnefsConn
	LastCleanup time.Time
}

type backendCfg struct {
	BypassCert     bool
	CleanupPeriod  int
	Endpoint       string
	HomeDir        string
	Password       string
	PrimaryGroup   string
	TTL            int
	TTLMax         int
	User           string
	UsernamePrefix string
}

var _ logical.Factory = Factory

// Factory returns a Hashicorp Vault secrets backend object
func Factory(ctx context.Context, cfg *logical.BackendConfig) (logical.Backend, error) {
	b := &backend{}
	b.LastCleanup = time.Now()
	b.Backend = &framework.Backend{
		BackendType: logical.TypeLogical,
		Help:        strings.TrimSpace(backendHelp),
		Paths: framework.PathAppend(
			pathConfigBuild(b),
			pathRolesDynamicBuild(b),
			pathRolesPredefinedBuild(b),
			pathCredsDynamicBuild(b),
			pathCredsPredefinedBuild(b),
		),
		InitializeFunc: b.pluginInit,
		PeriodicFunc:   b.pluginPeriod,
		Clean:          b.pluginCleanup,
	}
	if err := b.Setup(ctx, cfg); err != nil {
		b.Logger().Info(fmt.Sprintf("Error during setup: %s", err))
		return nil, err
	}
	return b, nil
}

func (b *backend) pluginInit(ctx context.Context, req *logical.InitializationRequest) error {
	cfg, err := getCfgFromStorage(ctx, req.Storage)
	if err != nil {
		return err
	}
	b.Conn = papi.NewPapiConn()
	if cfg == nil {
		b.Logger().Info("No configuration found. Configure this plugin at the URL <plugin_path>/config/root")
		return nil
	}
	err = b.Conn.PapiConnect(&papi.OnefsCfg{
		User:       cfg.User,
		Password:   cfg.Password,
		Endpoint:   cfg.Endpoint,
		BypassCert: cfg.BypassCert,
	})
	if err != nil {
		b.Logger().Info(fmt.Sprintf("Unable to connect to endpoint during plugin creation: %s", err))
	}
	return nil
}

func (b *backend) pluginPeriod(ctx context.Context, req *logical.Request) error {
	cfg, err := getCfgFromStorage(ctx, req.Storage)
	if err != nil || cfg == nil {
		return nil
	}
	// Wait until we have a valid config
	if cfg.CleanupPeriod <= 0 {
		return nil
	}
	// Use the stored last cleanup time and only after the configured cleanup time is exceeded do we query all users and perform cleanup
	cleanupTime := b.LastCleanup.Add(time.Second * time.Duration(cfg.CleanupPeriod))
	curTime := time.Now()
	if curTime.After(cleanupTime) {
		rex := regexp.MustCompile(fmt.Sprintf(defaultUserRegexp, cfg.UsernamePrefix))
		zones, err := b.Conn.PapiGetAccessZoneList()
		if err != nil {
			b.Logger().Error(fmt.Sprintf("[pluginPeriod] Unable to get access zone list: %s", err))
			return err
		}
		for i := 0; i < len(zones); i++ {
			users, err := b.Conn.PapiGetUserList(zones[i].Name)
			if err != nil {
				b.Logger().Error(fmt.Sprintf("[pluginPeriod] Unable to get user list for access zone: %s", zones[i].Name))
				continue
			}
			for j := 0; j < len(users); j++ {
				// Regex match each user name to determine which users are created by this plugin
				result := rex.FindAllStringSubmatch(users[j].Name, -1)
				if result != nil {
					// If the user name matches, we need to parse the expiration timestamp from the user name and compare it to the current time
					expireTime, err := time.ParseInLocation(defaultPathCredsDynamicTimeFormat, result[0][1], time.Local)
					if err != nil {
						return err
					}
					// If expireTime is earlier than our current time then this user has expired
					if expireTime.Before(curTime) {
						_, err := b.Conn.PapiDeleteUser(users[j].Name, zones[i].Name)
						if err != nil {
							b.Logger().Error(fmt.Sprintf("[pluginPeriod] Unable to delete user %s for access zone: %s", users[j].Name, zones[i].Name))
						}
					}
				}
			}
		}
		// TODO: We should increment a multiple of LastCleanup. Just using the current time can lead to cleanup time drift.
		b.LastCleanup = curTime
	}
	return nil
}

func (b *backend) pluginCleanup(ctx context.Context) {
	if b.Conn != nil {
		b.Conn.PapiDisconnect()
	}
}
