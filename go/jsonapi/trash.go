package jsonapi

import (
	"net/http"
	"time"

	"github.com/go-chi/render"

	"go.5x5.cz/inventario/models"
)

// TrashEntryMeta carries what the Trash page shows next to an entry.
type TrashEntryMeta struct {
	// DeletedByName is the name of the member who deleted the entry, empty
	// when that user no longer exists.
	DeletedByName string `json:"deleted_by_name,omitempty"`
	// ExpiresAt is when the entry is deleted for good.
	ExpiresAt time.Time `json:"expires_at"`
}

// TrashedCommodityMeta is TrashEntryMeta plus the files that went to Trash
// with the item.
type TrashedCommodityMeta struct {
	TrashEntryMeta
	FileCount int   `json:"file_count" example:"2" format:"int64"`
	FileBytes int64 `json:"file_bytes" example:"204800" format:"int64"`
}

// TrashedCommodityData is one item in Trash.
type TrashedCommodityData struct {
	ID         string               `json:"id"`
	Type       string               `json:"type" example:"commodities" enums:"commodities"`
	Attributes *models.Commodity    `json:"attributes"`
	Meta       TrashedCommodityMeta `json:"meta"`
}

// TrashedFileData is one file in Trash.
type TrashedFileData struct {
	ID         string             `json:"id"`
	Type       string             `json:"type" example:"files" enums:"files"`
	Attributes *models.FileEntity `json:"attributes"`
	Meta       TrashEntryMeta     `json:"meta"`
}

// TrashListMeta is the pagination block of a Trash list.
type TrashListMeta struct {
	Total      int `json:"total" example:"1" format:"int64"`
	Page       int `json:"page" example:"1" format:"int64"`
	PerPage    int `json:"per_page" example:"50" format:"int64"`
	TotalPages int `json:"total_pages" example:"1" format:"int64"`
	// RetentionDays is how many days an entry stays in Trash.
	RetentionDays int `json:"retention_days" example:"30" format:"int64"`
}

// TrashedCommoditiesResponse is a page of items in Trash.
type TrashedCommoditiesResponse struct {
	Data []TrashedCommodityData `json:"data"`
	Meta TrashListMeta          `json:"meta"`
}

// Render satisfies render.Renderer.
func (*TrashedCommoditiesResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// TrashedFilesResponse is a page of files in Trash.
type TrashedFilesResponse struct {
	Data []TrashedFileData `json:"data"`
	Meta TrashListMeta     `json:"meta"`
}

// Render satisfies render.Renderer.
func (*TrashedFilesResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// NewTrashListMeta builds the pagination block.
func NewTrashListMeta(total, page, perPage int, retention time.Duration) TrashListMeta {
	return TrashListMeta{
		Total:         total,
		Page:          page,
		PerPage:       perPage,
		TotalPages:    ComputeTotalPages(total, perPage),
		RetentionDays: int(retention / (24 * time.Hour)),
	}
}

// CommodityRestoreMeta tells the caller what restoring an item changed.
type CommodityRestoreMeta struct {
	// AreaRemoved is true when the item's area was deleted while the item
	// was in Trash; the item came back without an area.
	AreaRemoved bool `json:"area_removed"`
	// FilesRestored counts the files that came back with the item.
	FilesRestored int `json:"files_restored" example:"2" format:"int64"`
}

// CommodityRestoreResponse is the restored item.
type CommodityRestoreResponse struct {
	Data *CommodityResponseData `json:"data"`
	Meta CommodityRestoreMeta   `json:"meta"`
}

// Render satisfies render.Renderer.
func (*CommodityRestoreResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// NewCommodityRestoreResponse wraps a restored item.
func NewCommodityRestoreResponse(c *models.Commodity, meta CommodityRestoreMeta) *CommodityRestoreResponse {
	return &CommodityRestoreResponse{
		Data: &CommodityResponseData{ID: c.ID, Type: "commodities", Attributes: c},
		Meta: meta,
	}
}

// TrashBulkRestoreResponse is the outcome of restoring several items or
// files.
type TrashBulkRestoreResponse struct {
	Data *TrashBulkRestoreData `json:"data"`
}

// TrashBulkRestoreData carries the per-id outcome.
type TrashBulkRestoreData struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Attributes *TrashBulkRestoreAttrs `json:"attributes"`
}

// TrashBulkRestoreAttrs lists which ids came back and which did not.
type TrashBulkRestoreAttrs struct {
	Succeeded []string         `json:"succeeded"`
	Failed    []BulkResultFail `json:"failed"`
	// AreaRemoved lists the restored items whose area was deleted while
	// they were in Trash; they came back without an area.
	AreaRemoved []string `json:"area_removed"`
}

// Render satisfies render.Renderer.
func (*TrashBulkRestoreResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// NewTrashBulkRestoreResponse wraps the outcome of a bulk restore.
func NewTrashBulkRestoreResponse(entityType string, succeeded []string, failed []BulkResultFail, areaRemoved []string) *TrashBulkRestoreResponse {
	if succeeded == nil {
		succeeded = []string{}
	}
	if failed == nil {
		failed = []BulkResultFail{}
	}
	if areaRemoved == nil {
		areaRemoved = []string{}
	}
	return &TrashBulkRestoreResponse{
		Data: &TrashBulkRestoreData{
			Type: entityType + ".bulk_restore_result",
			ID:   "bulk-result",
			Attributes: &TrashBulkRestoreAttrs{
				Succeeded:   succeeded,
				Failed:      failed,
				AreaRemoved: areaRemoved,
			},
		},
	}
}

// EmptyTrashResponse reports what emptying Trash deleted.
type EmptyTrashResponse struct {
	Data *EmptyTrashData `json:"data"`
}

// EmptyTrashData is the body of EmptyTrashResponse.
type EmptyTrashData struct {
	Type       string           `json:"type" example:"trash" enums:"trash"`
	ID         string           `json:"id"`
	Attributes *EmptyTrashAttrs `json:"attributes"`
}

// EmptyTrashAttrs counts what was deleted.
type EmptyTrashAttrs struct {
	Commodities int `json:"commodities" example:"3" format:"int64"`
	Files       int `json:"files" example:"1" format:"int64"`
}

// Render satisfies render.Renderer.
func (*EmptyTrashResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// NewEmptyTrashResponse wraps the counts.
func NewEmptyTrashResponse(commodities, files int) *EmptyTrashResponse {
	return &EmptyTrashResponse{
		Data: &EmptyTrashData{
			Type:       "trash",
			ID:         "trash",
			Attributes: &EmptyTrashAttrs{Commodities: commodities, Files: files},
		},
	}
}
