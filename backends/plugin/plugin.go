/*
 * This file is part of remco.
 * © 2016 The Remco Authors
 *
 * For the full copyright and license information, please view the LICENSE
 * file that was distributed with this source code.
 */

package plugin

import (
	"context"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"path"

	"github.com/HeavyHorst/easyKV"
	berr "github.com/HeavyHorst/remco/backends/error"
	"github.com/HeavyHorst/remco/template"
	"github.com/natefinch/pie"
)

type plug struct {
	client *rpc.Client
}

// Plugin represents the config for a plugin.
type Plugin struct {
	// the path to the plugin executable
	Path   string
	Config map[string]interface{}
	template.Backend
}

// Connect creates the connection to the plugin and sends the config map to the same.
func (p *Plugin) Connect() (template.Backend, error) {
	if p == nil {
		return template.Backend{}, berr.ErrNilConfig
	}

	p.Backend.Name = path.Base(p.Path)

	client, err := pie.StartProviderCodec(jsonrpc.NewClientCodec, os.Stderr, p.Path)
	if err != nil {
		return p.Backend, err
	}

	plugin := &plug{client}
	if err := plugin.Init(p.Config); err != nil {
		return p.Backend, err
	}

	p.Backend.ReadWatcher = plugin
	return p.Backend, nil
}

// Init sends the config map to the plugin
// the plugin can then run some initialization tasks
func (p *plug) Init(config map[string]interface{}) error {
	var result bool
	return p.client.Call("Plugin.Init", config, &result)
}

// GetValues queries the plugin for keys
func (p *plug) GetValues(keys []string) (result map[string]string, err error) {
	err = p.client.Call("Plugin.GetValues", keys, &result)
	return result, err
}

// Close closes the client connection
func (p *plug) Close() {
	p.client.Call("Plugin.Close", nil, nil)
	p.client.Close()
}

type WatchConfig struct {
	Prefix string
	Opts   easyKV.WatchOptions
}

// WatchPrefix is not supported for now
func (p *plug) WatchPrefix(prefix string, ctx context.Context, opts ...easyKV.WatchOption) (uint64, error) {
	var result uint64

	wc := WatchConfig{Prefix: prefix}
	for _, option := range opts {
		option(&wc.Opts)
	}

	errchan := make(chan error)
	go func() {
		select {
		case errchan <- p.client.Call("Plugin.WatchPrefix", wc, &result):
		case <-ctx.Done():
		}
	}()

	select {
	case <-ctx.Done():
		return wc.Opts.WaitIndex, nil
	case err := <-errchan:
		return result, err
	}
}
