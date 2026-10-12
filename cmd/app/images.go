package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amplia/jira8/internal/markup"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

// Body is a description or comment ready to send: Wiki Markup, plus the local
// image files it references when it came as Markdown.
type Body struct {
	Text   string
	Set    bool
	Images []string
}

// ReadBody reads a text flag pair (see ReadTextInput) and, with --markdown,
// converts it to Wiki Markup. Local images (![alt](path)) become !name! and
// are returned in Images, checked to exist: upload them with UploadImages.
// Relative paths resolve against the Markdown file's directory, or the working
// directory for inline text and stdin.
func ReadBody(cmd *cobra.Command, inlineFlag, fileFlag string) (Body, error) {
	text, set, err := ReadTextInput(cmd, inlineFlag, fileFlag)
	if err != nil || !set {
		return Body{Text: text, Set: set}, err
	}
	if md, _ := cmd.Flags().GetBool("markdown"); !md {
		return Body{Text: text, Set: true}, nil
	}
	base := "."
	if f, _ := cmd.Flags().GetString(fileFlag); f != "" && f != "-" {
		base = filepath.Dir(f)
	}
	images, err := localImageFiles(markup.LocalImages(text), base)
	if err != nil {
		return Body{}, err
	}
	return Body{Text: markup.MarkdownToWiki(text), Set: true, Images: images}, nil
}

// localImageFiles resolves image paths and checks them before anything is
// written to Jira. Two different files with the same name are rejected: in
// Wiki Markup both would read !name!.
func localImageFiles(srcs []string, base string) ([]string, error) {
	var files []string
	byName := map[string]string{}
	for _, src := range srcs {
		p := src
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, p)
		}
		fi, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("image %s: %w", src, err)
		}
		if fi.IsDir() {
			return nil, fmt.Errorf("image %s is a directory", src)
		}
		name := filepath.Base(p)
		if prev, ok := byName[name]; ok && prev != p {
			return nil, fmt.Errorf("images %s and %s share the name %s: rename one", prev, p, name)
		}
		if _, ok := byName[name]; !ok {
			byName[name] = p
			files = append(files, p)
		}
	}
	return files, nil
}

// UploadImages attaches files to the issue, skipping those already attached
// with the same name and size, so re-sending a description does not duplicate
// its images.
func UploadImages(ctx context.Context, key string, files []string) ([]models.Attachment, error) {
	if len(files) == 0 {
		return nil, nil
	}
	c := Get().Client
	existing, err := c.ListAttachments(ctx, key)
	if err != nil {
		return nil, err
	}
	have := map[string]int64{}
	for _, a := range existing {
		have[a.Filename] = a.Size
	}
	var todo []string
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			return nil, err
		}
		if size, ok := have[filepath.Base(f)]; ok && size == fi.Size() {
			continue
		}
		todo = append(todo, f)
	}
	if len(todo) == 0 {
		return nil, nil
	}
	uploaded, err := c.AddAttachments(ctx, key, todo)
	if err != nil {
		return nil, fmt.Errorf("uploading images to %s: %w", key, err)
	}
	return uploaded, nil
}

// PrintUploadedImages lists, for humans, the images UploadImages attached.
func PrintUploadedImages(atts []models.Attachment) {
	for _, a := range atts {
		fmt.Printf("  image %s attached (#%s)\n", a.Filename, a.ID)
	}
}
