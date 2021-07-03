package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/diamondburned/wyzenip/internal/bulb"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

//go:embed window.glade
var gladeWindow string

func init() {
	gtk.Init(nil)
}

func main() {
	b, _ := gtk.BuilderNew()
	if err := b.AddFromString(gladeWindow); err != nil {
		log.Fatalln("builder fail:", err)
	}

	gladeObj := func(id string) glib.IObject {
		o, err := b.GetObject(id)
		if err != nil {
			log.Panicln("missing object ID", id)
		}
		return o
	}

	pager := gladeObj("pager").(*gtk.Stack)
	pager.SetVisibleChildName("connect")

	// page 1: connect
	entryEndpoint := gladeObj("entry-endpoint").(*gtk.Entry)
	entryUsername := gladeObj("entry-username").(*gtk.Entry)
	entryPassword := gladeObj("entry-password").(*gtk.Entry)
	entryTopic := gladeObj("entry-topic").(*gtk.Entry)
	buttonConnect := gladeObj("button-connect").(*gtk.Button)
	labelConnectError := gladeObj("label-connect-error").(*gtk.Label)

	if oldURL := loadURL(); oldURL != nil {
		entryEndpoint.SetText(oldURL.Scheme + "://" + oldURL.Host)
		entryUsername.SetText(oldURL.User.Username())
		entryTopic.SetText(strings.TrimPrefix(oldURL.Path, "/"))
	}

	// page 2: brightness
	scaleBrightness := gladeObj("scale-brightness").(*gtk.Scale)
	labelBrightnessError := gladeObj("label-brightness-error").(*gtk.Label)

	topWindow := gladeObj("top-window").(*gtk.Window)
	topWindow.Connect("destroy", gtk.MainQuit)
	topWindow.Show()

	setErrorLabel := func(l *gtk.Label, errText string) {
		l.SetMarkup(fmt.Sprintf(
			`<span color="red"><b>Error:</b> %s</span>`, errText,
		))
	}

	var client *bulb.MQTTClient
	var dimmer *bulb.Dimmer

	buttonConnect.Connect("clicked", func() {
		if !strings.Contains(mustText(entryEndpoint), "://") {
			setErrorLabel(labelConnectError, "endpoint missing schema, must have mqtt://")
			return
		}

		u, err := url.Parse(mustText(entryEndpoint))
		if err != nil {
			setErrorLabel(labelConnectError, err.Error())
			return
		}

		u.User = url.UserPassword(mustText(entryUsername), mustText(entryPassword))
		u.Path = path.Join("/", mustText(entryTopic))

		topWindow.SetSensitive(false)

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			c, err := bulb.Connect(ctx, u.String())
			if err == nil {
				saveURL(u)
			}

			glib.IdleAdd(func() {
				defer topWindow.SetSensitive(true)

				if err != nil {
					setErrorLabel(labelConnectError, err.Error())
					return
				}

				d := bulb.NewDimmer(c)
				d.SetBlocking(true)

				dimmer = &d
				client = c

				topWindow.Connect("destroy", client.Disconnect)
				pager.SetVisibleChildName("brightness")
			})
		}()
	})

	scaleBrightness.Connect("value-changed", func(scaleBrightness *gtk.Scale) {
		if err := dimmer.Dim(context.Background(), int(scaleBrightness.GetValue())); err != nil {
			setErrorLabel(labelBrightnessError, err.Error())
		}
	})

	gtk.Main()
}

func mustText(entry *gtk.Entry) string {
	t, err := entry.GetText()
	if err != nil {
		log.Panicln("failed to get text:", err)
	}
	return t
}
