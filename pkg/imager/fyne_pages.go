package imager

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/theme"
	"fyne.io/fyne/widget"
	"github.com/SkycoinProject/dmsg/cipher"

	"github.com/SkycoinProject/skybian/pkg/boot"
)

func (fg *FyneGUI) Page1() fyne.CanvasObject {
	title := "Welcome to Skyimager!"
	body := "This tool will:\n\n" +
		"1. Download a base image of Skybian.\n" +
		"2. Prepare an array of boot parameters.\n" +
		"3. Generate final images with provided boot parameters.\n" +
		"4. Provide instructions on how to flash final images.\n"

	conf := pageConfig{
		I:    1,
		Name: "Introduction",
		Next: func() { fg.w.SetContent(fg.Page2()) },
	}
	return makePage(conf, widget.NewVBox(
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(body, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})))
}

func (fg *FyneGUI) Page2() fyne.CanvasObject {
	wkDir := newLinkedEntry(&fg.wkDir)

	baseImgs, latestImg := fg.listBaseImgs()
	baseImg := widget.NewSelect(baseImgs, func(s string) {
		fg.baseImg = s
		fg.log.Debugf("Set: fg.baseImg = %v", s)
	})
	if baseImg.Selected = fg.baseImg; baseImg.Selected == "" && len(baseImg.Options) > 0 {
		baseImg.SetSelected(latestImg)
	}

	gwIP := newEntry(fg.gwIP.String(), func(s string) {
		fg.gwIP = net.ParseIP(s)
		fg.log.Debugf("Set: fg.gwIP = %v", s)
	})

	socksPC := newLinkedEntry(&fg.socksPC)
	socksPC.SetPlaceHolder("passcode")

	visors := newEntry(strconv.Itoa(fg.visors), func(s string) {
		fg.visors, _ = strconv.Atoi(s)
		fg.log.Debugf("Set: fg.visors = %v", s)
	})

	genHvImg := widget.NewCheck("Generate Hypervisor Image.", func(b bool) {
		fg.hvImg = b
		fg.log.Debugf("Set: fg.genHvImg = %v", b)
	})
	genHvImg.SetChecked(fg.hvImg)

	hvPKs := widget.NewVBox()
	hvPKs.Hide()
	hvPKsRefresh := func() {
		hvPKs.Children = nil
		for _, pk := range fg.hvPKs {
			hvPKs.Append(widget.NewLabelWithStyle(pk.String(),
				fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}))
		}
	}

	hvPKsAdd := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		title := "Trusted Hypervisors"
		confirm := "Add"
		dismiss := "Cancel"
		input := widget.NewEntry()
		input.SetPlaceHolder("public key")
		cont := fyne.NewContainerWithLayout(layout.NewVBoxLayout(),
			widget.NewLabel("Add trusted hypervisor public key:"), input)
		dialog.ShowCustomConfirm(title, confirm, dismiss, cont, func(b bool) {
			if !b {
				return
			}
			var pk cipher.PubKey
			if err := pk.Set(input.Text); err != nil {
				showErr(fg, fmt.Errorf("failed to add public key: %v", err))
				return
			}
			for _, oldPK := range fg.hvPKs {
				if pk == oldPK {
					showErr(fg, fmt.Errorf("public key '%s' is already added", pk))
					return
				}
			}
			fg.hvPKs = append(fg.hvPKs, pk)
			hvPKsRefresh()
		}, fg.w)
	})
	hvPKsAdd.Hide()

	enableHvPKs := widget.NewCheck("Manually Add Trusted Hypervisors.", func(b bool) {
		if b {
			hvPKsRefresh()
			hvPKs.Show()
			hvPKsAdd.Show()
		} else {
			fg.hvPKs = nil
			hvPKs.Hide()
			hvPKsAdd.Hide()
		}
	})
	enableHvPKs.SetChecked(len(fg.hvPKs) > 0)

	conf := pageConfig{
		I:    2,
		Name: "Prepare Boot Parameters",
		Reset: func() {
			fg.resetPage2Values()
			fg.w.SetContent(fg.Page2())
		},
		Prev: func() { fg.w.SetContent(fg.Page1()) },
		Next: func() {
			if !checkPage2Inputs(fg, visors.Text) {
				return
			}
			confirmPage2Continue(fg, fg.wkDir, func() {
				bpsStr, err := fg.generateBPS()
				if err != nil {
					dialog.ShowError(err, fg.w)
					return
				}
				fg.w.SetContent(fg.Page3(bpsStr))
			})
		},
	}
	return makePage(conf,
		widget.NewLabel("Work Directory:"), wkDir,
		widget.NewLabel("Base Image:"), baseImg,
		widget.NewLabel("Gateway IP:"), gwIP,
		widget.NewLabel("Skysocks Passcode:"), socksPC,
		widget.NewLabel("Number of Visor Images:"), visors,
		genHvImg, enableHvPKs, hvPKs, hvPKsAdd)
}

func (fg *FyneGUI) resetPage2Values() {
	fg.wkDir = DefaultRootDir()
	fg.baseImg = ""
	fg.gwIP = net.ParseIP(boot.DefaultGatewayIP)
	fg.socksPC = ""
	fg.visors = DefaultVCount
	fg.hvImg = true
	fg.hvPKs = nil
}

func checkPage2Inputs(fg *FyneGUI, visorsText string) bool {
	if _, err := filepath.Abs(fg.wkDir); err != nil {
		return showErr(fg, fmt.Errorf("invalid Work Directory: %v", err))
	}
	if strings.TrimSpace(fg.baseImg) == "" {
		return showErr(fg, errors.New("invalid Base Image URL: cannot be empty"))
	}
	if fg.gwIP == nil {
		return showErr(fg, fmt.Errorf("invalid Gateway IP"))
	}
	if _, err := strconv.Atoi(visorsText); err != nil {
		return showErr(fg, fmt.Errorf("invalid Number of Visor Images: %v", err))
	}
	if fg.visors < 0 {
		return showErr(fg, fmt.Errorf("cannot create %d Visor Images", fg.visors))
	}
	return true
}

func confirmPage2Continue(fg *FyneGUI, wkDir string, next func()) {
	cTitle := "Work Directory Already Exists"
	cMsg := fmt.Sprintf("Directory %s already exists.\nDelete everything and continue?", wkDir)
	dialog.ShowConfirm(cTitle, cMsg, func(b bool) {
		if !b {
			showErr(fg)
			return
		}
		if err := os.RemoveAll(wkDir); err != nil {
			showErr(fg, fmt.Errorf("failed to clear work directory: %v", err))
			return
		}
		dialog.ShowInformation("Information", "Work directory cleared.", fg.w)
		next()
	}, fg.w)
}

func (fg *FyneGUI) Page3(bpsStr string) fyne.CanvasObject {
	bps := widget.NewMultiLineEntry()
	bps.SetText(bpsStr)

	conf := pageConfig{
		I:         3,
		Name:      "Finalize Boot Parameters",
		Prev:      func() { fg.w.SetContent(fg.Page2()) },
		ResetText: "Regenerate",
		Reset: func() {
			bpsStr, err := fg.generateBPS()
			if err != nil {
				dialog.ShowError(err, fg.w)
				return
			}
			bps.SetText(bpsStr)
		},
		NextText: "Download and Build",
		Next: func() {
			// Decode bps entry text to ensure changes are recorded.
			dec := json.NewDecoder(strings.NewReader(bps.Text))
			if err := dec.Decode(&fg.bps); err != nil {
				dialog.ShowError(fmt.Errorf("invalid boot paramters: %v", err), fg.w)
				return
			}
			dialog.ShowConfirm("Confirmation", "Start download and build?", func(b bool) {
				if b {
					fg.build()
				}
			}, fg.w)
		},
	}
	return makePage(conf, bps)
}
