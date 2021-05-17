package gopherforms

import (
	"bytes"
	"encoding/json"
	"github.com/df-mc/dragonfly/dragonfly/player/form"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"go.uber.org/atomic"
	"strings"
	"sync"
)

// User is a user that is connected over Gophertunnel.
// It is used to contain important session data, like the end-server form ID and the user form ID.
type User struct {
	mu           *sync.Mutex
	forms        map[uint32]form.Form
	conn         *minecraft.Conn
	localFormId  *atomic.Uint32
	remoteFormId *atomic.Uint32
}

// nullBytes contains the word 'null' converted to a byte slice.
var nullBytes = []byte("null\n")

// NewUser returns a new user.
func NewUser(conn *minecraft.Conn) *User {
	return &User{
		mu:           &sync.Mutex{},
		forms:        make(map[uint32]form.Form),
		conn:         conn,
		localFormId:  atomic.NewUint32(0),
		remoteFormId: atomic.NewUint32(0),
	}
}

// Conn returns the user connection.
func (u *User) Conn() *minecraft.Conn {
	return u.conn
}

// Remote returns the remote form ID.
func (u *User) Remote() uint32 {
	return u.remoteFormId.Load()
}

// Local returns the local form ID.
func (u *User) Local() uint32 {
	return u.localFormId.Load()
}

// HandleForm handles a form and checks if it was gophertunnel side.
// If gophertunnel handled the form, it returns true.
func (u *User) HandleForm(pk *packet.ModalFormResponse) bool {
	u.mu.Lock()
	if f, ok := u.forms[pk.FormID]; ok {
		delete(u.forms, pk.FormID)
		u.mu.Unlock()

		if bytes.Equal(pk.ResponseData, nullBytes) || len(pk.ResponseData) == 0 {
			return true
		}
		if !ok {
			return false
		}
		if err := f.SubmitJSON(pk.ResponseData, u); err != nil {
			return false
		}

		return true
	}

	return false
}

// SendForm sends a Dragonfly form to a gophertunnel user.
func (u *User) SendForm(f form.Form) {
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

	u.mu.Lock()
	if len(u.forms) > 10 {
		for k := range u.forms {
			delete(u.forms, k)
			break
		}
	}
	u.localFormId.Add(1)

	id := u.localFormId.Load()
	u.forms[id] = f
	u.mu.Unlock()

	u.conn.WritePacket(&packet.ModalFormRequest{
		FormID:   id,
		FormData: b,
	})
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
