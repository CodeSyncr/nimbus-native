package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/camera"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Review takes a photo or picks one, ready to upload.
type Review struct{}

func describe(p camera.Photo, err error) (string, error) {
	if plugin.IsCode(err, plugin.Cancelled) {
		return "", nil
	}
	return fmt.Sprintf("%d×%d, %d KB", p.Width, p.Height, len(p.Data)/1024), err
}

func takePhoto(ctx context.Context, _ struct{}) (string, error) {
	return describe(camera.Take(ctx, camera.Options{MaxSize: 1600}))
}

func pickPhoto(ctx context.Context, _ struct{}) (string, error) {
	return describe(camera.Pick(ctx, camera.Options{MaxSize: 1600}))
}

func pickSeveral(ctx context.Context, _ struct{}) (string, error) {
	photos, err := camera.PickMany(ctx, camera.Options{MaxSize: 1600}, 5)
	if plugin.IsCode(err, plugin.Cancelled) {
		return "", nil
	}
	return fmt.Sprintf("%d photos", len(photos)), err
}

func pickClip(ctx context.Context, _ struct{}) (string, error) {
	v, err := camera.PickVideo(ctx, camera.VideoOptions{})
	if plugin.IsCode(err, plugin.Cancelled) {
		return "", nil
	}
	return fmt.Sprintf("%.0f seconds, %s, %d KB", v.Seconds, v.Type, len(v.Data)/1024), err
}

func (Review) Render(c *ui.Ctx) ui.Component {
	take, pick := ui.UseAction(c, takePhoto), ui.UseAction(c, pickPhoto)
	several, clip := ui.UseAction(c, pickSeveral), ui.UseAction(c, pickClip)
	status := take.Result + pick.Result + several.Result + clip.Result
	if take.Err != nil {
		status = take.Err.Error()
	}
	if pick.Err != nil {
		status = pick.Err.Error()
	}
	if several.Err != nil {
		status = several.Err.Error()
	}
	if clip.Err != nil {
		status = clip.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Take a photo", OnPress: func() { take.Run(struct{}{}) }},
		ui.Button{Label: "Choose from library", Variant: ui.Tonal, OnPress: func() { pick.Run(struct{}{}) }},
		ui.Button{Label: "Choose several", Variant: ui.Tonal, OnPress: func() { several.Run(struct{}{}) }},
		ui.Button{Label: "Choose a video", Variant: ui.Tonal, OnPress: func() { clip.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
