package storage

import (
	"all-me-backend/pkg/models"
	"errors"
	"fmt"
	"io"
	"testing"
)

// fakeProvider serves a folder tree: folders[id] lists the children of folder id.
type fakeProvider struct {
	folders map[string][]*models.CloudItem
	broken  map[string]bool
}

func (p *fakeProvider) ListFolderContents(item *models.CloudItem, _ *models.Token, _ int, _ string) ([]*models.CloudItem, string, error) {
	if p.broken[item.ID] {
		return nil, "", errors.New("no access")
	}
	return p.folders[item.ID], "", nil
}
func (p *fakeProvider) GetFileStream(*models.CloudItem, *models.Token) (io.ReadCloser, error) {
	return nil, nil
}
func (p *fakeProvider) GetFaceRecognitionOptimizedStream(*models.CloudItem, *models.Token) (io.ReadCloser, error) {
	return nil, nil
}
func (p *fakeProvider) ParseShareLink(string, *models.Token) (*models.CloudItem, error) {
	return nil, nil
}

func folder(id string) *models.CloudItem { return &models.CloudItem{ID: id, Name: id, IsFolder: true} }
func image(id string) *models.CloudItem {
	return &models.CloudItem{ID: id, Name: id + ".jpg", MimeType: "image/jpeg"}
}

func listImages(t *testing.T, p *fakeProvider) ([]*models.CloudItem, error) {
	t.Helper()
	s := NewService(p, p)
	return s.ListImages(folder("root"), &models.Token{Provider: "googledrive"}, true)
}

func TestListImages_SkipsFoldersBelowMaxDepth(t *testing.T) {
	p := &fakeProvider{folders: map[string][]*models.CloudItem{}}
	// root -> d1 -> ... -> d(MaxFolderDepth+1), with one image in every folder
	parent := "root"
	for depth := 0; depth <= MaxFolderDepth+1; depth++ {
		p.folders[parent] = append(p.folders[parent], image(fmt.Sprintf("img%d", depth)))
		child := fmt.Sprintf("d%d", depth+1)
		p.folders[parent] = append(p.folders[parent], folder(child))
		parent = child
	}

	images, err := listImages(t, p)
	if err != nil {
		t.Fatal(err)
	}
	// Images in root (depth 0) through depth MaxFolderDepth
	if len(images) != MaxFolderDepth+1 {
		t.Errorf("got %d images, want %d", len(images), MaxFolderDepth+1)
	}
}

func TestListImages_StopsAboveMaxImages(t *testing.T) {
	p := &fakeProvider{folders: map[string][]*models.CloudItem{"root": {folder("a"), folder("b")}}}
	for i := 0; i < MaxImagesPerSearch; i++ {
		p.folders["a"] = append(p.folders["a"], image(fmt.Sprint("a", i)))
	}

	if images, err := listImages(t, p); err != nil || len(images) != MaxImagesPerSearch {
		t.Fatalf("exactly at the limit: got %d images, err %v", len(images), err)
	}

	p.folders["b"] = []*models.CloudItem{image("one-too-many")}
	if _, err := listImages(t, p); !errors.Is(err, ErrTooManyImages) {
		t.Errorf("above the limit: got %v, want ErrTooManyImages", err)
	}
}

func TestListImages_SkipsUnreadableSubfolders(t *testing.T) {
	p := &fakeProvider{
		folders: map[string][]*models.CloudItem{"root": {folder("locked"), image("x")}},
		broken:  map[string]bool{"locked": true},
	}
	images, err := listImages(t, p)
	if err != nil || len(images) != 1 {
		t.Errorf("got %d images, err %v; want 1 image", len(images), err)
	}
}
