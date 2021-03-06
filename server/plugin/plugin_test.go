package plugin

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/bouk/monkey"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"github.com/mattermost/mattermost-server/plugin/plugintest"
	"github.com/matterpoll/matterpoll/server/store"
	"github.com/matterpoll/matterpoll/server/store/kvstore"
	"github.com/matterpoll/matterpoll/server/store/mockstore"
	"github.com/matterpoll/matterpoll/server/utils/testutils"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupTestPlugin(t *testing.T, api *plugintest.API, store *mockstore.Store, siteURL string) *MatterpollPlugin {
	defaultServerLocale := "en"
	p := &MatterpollPlugin{
		ServerConfig: &model.Config{
			ServiceSettings: model.ServiceSettings{
				SiteURL: &siteURL,
			},
			LocalizationSettings: model.LocalizationSettings{
				DefaultServerLocale: &defaultServerLocale,
			},
		},
	}
	p.setConfiguration(&configuration{
		Trigger: "poll",
	})

	p.SetAPI(api)
	p.Store = store
	p.router = p.InitAPI()
	p.bundle = &i18n.Bundle{}

	return p
}

func TestPluginOnActivate(t *testing.T) {
	for name, test := range map[string]struct {
		SetupAPI    func(*plugintest.API) *plugintest.API
		ShouldError bool
	}{
		// server version tests
		"greater minor version than minimumServerVersion": {
			SetupAPI: func(api *plugintest.API) *plugintest.API {
				m := semver.MustParse(minimumServerVersion)
				m.Minor++
				m.Patch = 0
				api.On("GetServerVersion").Return(m.String())

				path, err := filepath.Abs("../..")
				require.Nil(t, err)
				api.On("GetBundlePath").Return(path, nil)
				return api
			},
			ShouldError: false,
		},
		"same version as minimumServerVersion": {
			SetupAPI: func(api *plugintest.API) *plugintest.API {
				api.On("GetServerVersion").Return(minimumServerVersion)

				path, err := filepath.Abs("../..")
				require.Nil(t, err)
				api.On("GetBundlePath").Return(path, nil)
				return api
			},
			ShouldError: false,
		},
		"lesser minor version than minimumServerVersion": {
			SetupAPI: func(api *plugintest.API) *plugintest.API {
				m := semver.MustParse(minimumServerVersion)
				if m.Minor == 0 {
					m.Major--
					m.Minor = 0
					m.Patch = 0
				} else {
					m.Minor--
					m.Patch = 0
				}
				api.On("GetServerVersion").Return(m.String())
				return api
			},
			ShouldError: true,
		},
		"GetServerVersion not implemented, returns empty string": {
			SetupAPI: func(api *plugintest.API) *plugintest.API {
				api.On("GetServerVersion").Return("")
				return api
			},
			ShouldError: true,
		},
		// i18n bundle tests
		"GetBundlePath fails": {
			SetupAPI: func(api *plugintest.API) *plugintest.API {
				api.On("GetServerVersion").Return(minimumServerVersion)
				api.On("GetBundlePath").Return("", errors.New(""))
				return api
			},
			ShouldError: true,
		},
		"i18n directory doesn't exist ": {
			SetupAPI: func(api *plugintest.API) *plugintest.API {
				api.On("GetServerVersion").Return(minimumServerVersion)
				api.On("GetBundlePath").Return("/tmp", nil)
				return api
			},
			ShouldError: true,
		},
		/*
			"GetBundlePath fails": {
				SetupAPI: func(api *plugintest.API) *plugintest.API {
					api.On("GetServerVersion").Return(minimumServerVersion)
					api.On("GetBundlePath").Return("", errors.New(""))
					return api
				},
				ShouldError: true,
			},
		*/
	} {
		t.Run(name, func(t *testing.T) {
			api := test.SetupAPI(&plugintest.API{})
			defer api.AssertExpectations(t)

			patch := monkey.Patch(kvstore.NewStore, func(plugin.API, string) (store.Store, error) {
				return &mockstore.Store{}, nil
			})
			defer patch.Unpatch()

			p := &MatterpollPlugin{}
			p.setConfiguration(&configuration{
				Trigger: "poll",
			})
			p.SetAPI(api)
			err := p.OnActivate()

			if test.ShouldError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
	t.Run("NewStore() fails", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return(minimumServerVersion)
		defer api.AssertExpectations(t)

		patch := monkey.Patch(kvstore.NewStore, func(plugin.API, string) (store.Store, error) {
			return nil, &model.AppError{}
		})
		defer patch.Unpatch()

		p := &MatterpollPlugin{}
		p.setConfiguration(&configuration{
			Trigger: "poll",
		})
		p.SetAPI(api)
		err := p.OnActivate()

		assert.NotNil(t, err)
	})
}

func TestPluginOnDeactivate(t *testing.T) {
	t.Run("all fine", func(t *testing.T) {
		api := &plugintest.API{}
		p := setupTestPlugin(t, api, &mockstore.Store{}, testutils.GetSiteURL())
		api.On("UnregisterCommand", "", p.getConfiguration().Trigger).Return(nil)
		defer api.AssertExpectations(t)

		err := p.OnDeactivate()
		assert.Nil(t, err)
	})

	t.Run("UnregisterCommand fails", func(t *testing.T) {
		api := &plugintest.API{}
		p := setupTestPlugin(t, api, &mockstore.Store{}, testutils.GetSiteURL())
		api.On("UnregisterCommand", "", p.getConfiguration().Trigger).Return(&model.AppError{})
		defer api.AssertExpectations(t)

		err := p.OnDeactivate()
		assert.NotNil(t, err)
	})
}

func GetMockArgumentsWithType(typeString string, num int) []interface{} {
	ret := make([]interface{}, num)
	for i := 0; i < len(ret); i++ {
		ret[i] = mock.AnythingOfTypeArgument(typeString)
	}
	return ret
}
