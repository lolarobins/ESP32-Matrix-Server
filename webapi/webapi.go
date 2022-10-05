package webapi

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"lolarobins.ca/esp32-matrix-server/matrix"
)

func HTTPMainHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := os.ReadFile("web/main.html")
	if err != nil {
		fmt.Fprintf(w, "Matrix32: Could not find main.html template")
		return
	}
	response := string(resp)

	io.WriteString(w, response)
}

func HTTPSelectionHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := os.ReadFile("web/selection.html")
	if err != nil {
		fmt.Fprintf(w, "Matrix32: Could not find selection.html template")
		return
	}
	response := string(resp)

	// fill lists with panels
	panelopts := "<option name=\"n/a\">Choose a panel</option>"
	for _, panel := range matrix.Panels {
		panelopts += "<option value=\"" + panel.Id + "\">" + panel.Name + " (" + panel.Id + ")</option>"
	}
	response = strings.ReplaceAll(response, "{PANELS}", panelopts)

	// fill with current image files
	fileopts := "<option name=\"n/a\">Choose a file</option>"
	files, _ := os.ReadDir("uploads")
	for _, f := range files {
		fileopts += "<option value=\"" + f.Name() + "\">" + f.Name() + "</option>"
	}
	response = strings.ReplaceAll(response, "{FILES}", fileopts)

	// simple page loading
	if r.Method == "GET" {
		response = strings.ReplaceAll(response, "{MSG}", "")
		io.WriteString(w, response)
		return
	}

	// page submission
	if err := r.ParseMultipartForm(12 << 20); err != nil { // 12mb max
		response = strings.ReplaceAll(response, "{MSG}", "Error: "+err.Error())
		io.WriteString(w, response)
		return
	}

	// get panel
	panel, assigned := matrix.Panels[r.FormValue("panel")]
	if !assigned {
		response = strings.ReplaceAll(response, "{MSG}", "Error: panel not found")
		io.WriteString(w, response)
		return
	}

	if err = panel.FillImage("uploads/" + r.FormValue("file")); err != nil {
		response = strings.ReplaceAll(response, "{MSG}", "Error: "+err.Error())
		io.WriteString(w, response)
		return
	}

	response = strings.ReplaceAll(response, "{MSG}", "Showing '"+r.FormValue("file")+"' on panel "+panel.Name+" ("+panel.Id+")")
	io.WriteString(w, response)
}

func HTTPUploadHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := os.ReadFile("web/upload.html")
	if err != nil {
		fmt.Fprintf(w, "Matrix32: Could not find upload.html template")
		return
	}
	response := string(resp)

	// page loading
	if r.Method == "GET" {
		response = strings.ReplaceAll(response, "{MSG}", "")
		io.WriteString(w, response)
		return
	}

	// file uploading
	if err := r.ParseMultipartForm(12 << 20); err != nil { // 12mb max
		response = strings.ReplaceAll(response, "{MSG}", "Error in upload: "+err.Error())
		io.WriteString(w, response)
		return
	}

	file, handler, err := r.FormFile("data")
	if err != nil {
		response = strings.ReplaceAll(response, "{MSG}", "Error in upload: "+err.Error())
		io.WriteString(w, response)
		return
	}
	defer file.Close()

	name := r.FormValue("name")
	if name == "" {
		name = handler.Filename
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, file); err != nil {
		response = strings.ReplaceAll(response, "{MSG}", "Error in upload: "+err.Error())
		io.WriteString(w, response)
		return
	}

	os.Mkdir("uploads", 0777)
	os.WriteFile("uploads/"+name, buf.Bytes(), 0777)

	response = strings.ReplaceAll(response, "{MSG}", "Upload of "+name+" successful!")
	io.WriteString(w, response)

	println("Uploaded: " + name)
}
