// designed to interact with https://github.com/lolarobins/ESP32-Matrix-Controller
package matrix

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/gif"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fogleman/gg"
	"github.com/nfnt/resize"
)

type MatrixPanel struct {
	// name of the panel
	Name string `json:"name"`
	// address of the matrix display
	Address string `json:"address"`
	// width of the panel
	Width uint8 `json:"width"`
	// height of the panel
	Height uint8 `json:"height"`
	// context used when calling draw
	Context *gg.Context `json:"-"`

	// animation handling
	animation bool
	animmutex *sync.Mutex
}

var panels = make(map[string]*MatrixPanel)

func LoadPanels() error {
	// load panels
	if info, err := os.Stat("panels"); !os.IsNotExist(err) && info.IsDir() {
		files, err := os.ReadDir("panels")
		if err != nil {
			return err
		}

		for _, f := range files {
			// ignore dotfiles, non-directories
			if strings.HasPrefix(f.Name(), ".") || !f.IsDir() {
				continue
			}

			data, err := os.ReadFile("panels" + f.Name())
			if err != nil {
				println(f.Name() + ": " + err.Error())
			}

			panel := new(MatrixPanel)
			if err := json.Unmarshal(data, panel); err != nil {
				println(f.Name() + ": " + err.Error())
			}

			panels[(f.Name())] = panel
		}
	} else if !os.IsNotExist(err) {
		return err
	} else if err := os.Mkdir("panels", 0777); err != nil {
		return err
	}

	return nil
}

func NewPanel(address string, w uint8, h uint8) MatrixPanel {
	return MatrixPanel{
		Address:   address,
		Width:     w,
		Height:    h,
		Context:   gg.NewContext(int(w), int(h)),
		animmutex: &sync.Mutex{},
	}
}

func (m *MatrixPanel) FillImage(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(data)
	var img image.Image

	split := strings.Split(filepath, ".")

	switch strings.ToLower(split[len(split)-1]) { // extension getting

	case "gif":
		var gifimg *gif.GIF
		if gifimg, err = gif.DecodeAll(buf); err != nil {
			return err
		}

		m.RenderGIF(*gifimg)
		return nil
	default:
		m.animation = false // stop animation and wait for it to finish
		m.animmutex.Lock()
		if img, _, err = image.Decode(buf); err != nil {
			return err
		}
	}

	resized := resize.Resize(uint(m.Width), uint(m.Height), img, resize.Bilinear)
	m.Context.DrawImage(resized, 0, 0) // resize to fit display

	err = m.Draw()

	m.animmutex.Unlock()

	return err
}

func (m *MatrixPanel) RenderGIF(img gif.GIF) {
	m.animation = false
	go func() {
		m.animmutex.Lock()
		m.animation = true

		var current, last int64
		for i, loops := 0, 0; m.animation; {
			current = time.Now().UnixMilli()
			if i != 0 && (current-last) > int64(img.Delay[i-1]*10) && last != 0 {
				continue
			}

			if i == len(img.Image) {
				i = 0
				if img.LoopCount == loops && img.LoopCount != 0 {
					m.animation = false
				}

				loops++
			}

			resized := resize.Resize(uint(m.Width), uint(m.Height), img.Image[i], resize.Lanczos3)
			m.Context.DrawImage(resized, 0, 0) // resize to fit display

			m.Draw()

			last = time.Now().UnixMilli()
			i++
		}

		m.animmutex.Unlock()
	}()
}

// draw the current canvas to the screen
func (m *MatrixPanel) Draw() error {
	imgdata := make([]byte, (uint32(m.Width)*uint32(m.Height))*2) // w * h * 2 bytes each

	for x := 0; x < int(m.Width); x++ { // convert and fill array
		for y := 0; y < int(m.Height); y++ {
			r888, g888, b888, _ := m.Context.Image().At(x, y).RGBA()

			r565 := ((r888 >> 3) & 0x1f) << 11
			g565 := ((g888 >> 2) & 0x3f) << 5
			b565 := (b888 >> 3) & 0x1f

			vals := make([]byte, 2)
			binary.LittleEndian.PutUint16(vals, uint16(r565|g565|b565))

			imgdata[((y*64)+x)*2] = vals[0]
			imgdata[(((y*64)+x)*2)+1] = vals[1]
		}
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "server-upload.bin")
	io.Copy(part, bytes.NewBuffer(imgdata))
	writer.Close()

	resp, err := http.Post("http://"+m.Address+"/upload", writer.FormDataContentType(), body)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("draw returned non-OK status code")
	}

	return nil
}

// clears the screen of the panel
func (m *MatrixPanel) Clear() error {
	resp, err := http.Get("http://" + m.Address + "/clear")

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("clear returned non-OK status code")
	}

	return nil
}
