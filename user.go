package gopherforms

import (
	"encoding/json"
	"github.com/df-mc/dragonfly/dragonfly/player/form"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"go.uber.org/atomic"
	"strings"
)

// User is a user that is connected over Gophertunnel.
// It is used to contain important session data, like the end-server form ID and the user form ID.
type User struct {
	conn *minecraft.Conn
	localFormId *atomic.Uint32
	remoteFormId *atomic.Uint32
}

// NewUser returns a new user.
func NewUser(conn *minecraft.Conn) *User {
	return &User{
		conn:         conn,
		localFormId:  atomic.NewUint32(0),
		remoteFormId: atomic.NewUint32(0),
	}
}

// Remote returns the remote form ID.
func (u *User) Remote() uint32 {
	return u.remoteFormId.Load()
}

// Local returns the local form ID.
func (u *User) Local() uint32 {
	return u.localFormId.Load()
}

// SendForm sends a Dragonfly form to a gophertunnel user.
func (u *User) SendForm(f form.Form) error {
	var n []map[string]interface{}
	m := map[string]interface{}{}

	switch frm := f.(type) {
	case form.Custom:
		m["type"], m["title"] = "custom_form", frm.Title()
		for _, e := range frm.Elements() {
			n = append(n, elemToMap(e))
		}
		m["content"] = n
	case form.Menu:
		m["type"], m["title"], m["content"] = "form", frm.Title(), frm.Body()
		for _, button := range frm.Buttons() {
			v := map[string]interface{}{"text": button.Text}
			if button.Image != "" {
				buttonType := "path"
				if strings.HasPrefix(button.Image, "http:") || strings.HasPrefix(button.Image, "https:") {
					buttonType = "url"
				}
				v["image"] = map[string]interface{}{"type": buttonType, "data": button.Image}
			}
			n = append(n, v)
		}
		m["buttons"] = n
	case form.Modal:
		m["type"], m["title"], m["content"] = "modal", frm.Title(), frm.Body()
		buttons := frm.Buttons()
		m["button1"], m["button2"] = buttons[0].Text, buttons[1].Text
	}

	b, _ := json.Marshal(m)

	u.localFormId.Add(1)

	err := u.conn.WritePacket(&packet.ModalFormRequest{
		FormID:   u.localFormId.Load(),
		FormData: b,
	})

	if err != nil {
		return err
	}

	return nil
}

// elemToMap encodes a form element to its representation as a map to be encoded to JSON for the client.
func elemToMap(e form.Element) map[string]interface{} {
	switch element := e.(type) {
	case form.Toggle:
		return map[string]interface{}{
			"type":    "toggle",
			"text":    element.Text,
			"default": element.Default,
		}
	case form.Input:
		return map[string]interface{}{
			"type":        "input",
			"text":        element.Text,
			"default":     element.Default,
			"placeholder": element.Placeholder,
		}
	case form.Label:
		return map[string]interface{}{
			"type": "label",
			"text": element.Text,
		}
	case form.Slider:
		return map[string]interface{}{
			"type":    "slider",
			"text":    element.Text,
			"min":     element.Min,
			"max":     element.Max,
			"step":    element.StepSize,
			"default": element.Default,
		}
	case form.Dropdown:
		return map[string]interface{}{
			"type":    "dropdown",
			"text":    element.Text,
			"default": element.DefaultIndex,
			"options": element.Options,
		}
	case form.StepSlider:
		return map[string]interface{}{
			"type":    "step_slider",
			"text":    element.Text,
			"default": element.DefaultIndex,
			"steps":   element.Options,
		}
	}
	panic("should never happen")
}