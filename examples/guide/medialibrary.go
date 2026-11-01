package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/imagemanipulator"
	"github.com/CodeSyncr/nimbus-native/plugins/medialibrary"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// SaveToPhotos saves a picture, then reads back the newest one.
type SaveToPhotos struct{}

func savePicture(ctx context.Context, _ struct{}) (string, error) {
	jpeg, err := imagemanipulator.JPEG(sample(), 85)
	if err != nil {
		return "", err
	}
	if _, err := medialibrary.Save(ctx, medialibrary.Item{Name: "monsoon.jpg", Type: "image/jpeg", Data: jpeg}); err != nil {
		return "", err
	}
	return "Saved to your photos", nil
}

func readNewest(ctx context.Context, _ struct{}) (string, error) {
	ok, err := medialibrary.Request(ctx)
	if err != nil || !ok {
		return "Photos are off", err
	}
	list, err := medialibrary.List(ctx, 5)
	if err != nil || len(list) == 0 {
		return "No photos yet", err
	}
	b, err := medialibrary.Read(ctx, list[0].ID, 400)
	return fmt.Sprintf("%d×%d, a %d KB preview", list[0].Width, list[0].Height, len(b)/1024), err
}

func (SaveToPhotos) Render(c *ui.Ctx) ui.Component {
	save, read := ui.UseAction(c, savePicture), ui.UseAction(c, readNewest)
	status := read.Result
	if status == "" {
		status = save.Result
	}
	if save.Err != nil {
		status = save.Err.Error()
	}
	if read.Err != nil {
		status = read.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Save a picture", OnPress: func() { save.Run(struct{}{}) }},
		ui.Button{Label: "Read the newest", Variant: ui.Tonal, OnPress: func() { read.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
