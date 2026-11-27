package services

import (
	"context"
	"io"
	"log/slog"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/imageprocessor"
)

// thumbnailOrientationBatchSize bounds how many files one query of the
// thumbnail orientation pass returns.
const thumbnailOrientationBatchSize = 100

// BackfillThumbnailOrientation makes the thumbnails of JPEG photos again
// when their EXIF orientation turns them and their thumbnails may show the
// stored pixels unturned. It reads the start of every JPEG file not marked
// ThumbnailsOriented. A photo stored the right way up is only marked; a
// turned one with thumbnails gets them written again, which marks it. A
// turned one without thumbnails stays unmarked: GenerateThumbnails marks it
// when it writes them, and if a server of an older version writes them
// unturned meanwhile (during a rolling deploy), the next pass finds them.
//
// A blob that cannot be read or decoded is skipped and logged and stays
// unmarked, so the next start tries it again; the pass moves on by id, so
// one bad file cannot stall the rest. The pass is idempotent: a second run
// writes nothing.
func BackfillThumbnailOrientation(ctx context.Context, factorySet *registry.FactorySet, uploadLocation string) {
	if factorySet == nil || factorySet.FileRegistryFactory == nil || uploadLocation == "" {
		return
	}
	bucket, err := blob.OpenBucket(ctx, uploadLocation)
	if err != nil {
		slog.WarnContext(ctx, "thumbnail orientation backfill skipped: bucket unavailable", "error", err.Error())
		return
	}
	defer func() {
		if cerr := bucket.Close(); cerr != nil {
			slog.WarnContext(ctx, "thumbnail orientation backfill: failed to close bucket", "error", cerr.Error())
		}
	}()

	svc := NewFileService(factorySet, uploadLocation)
	reg := factorySet.FileRegistryFactory.CreateServiceRegistry()
	start := time.Now()
	checked, remade, failed := 0, 0, 0
	afterID := ""
	for {
		if ctx.Err() != nil {
			return
		}
		batch, err := reg.ListPendingThumbnailOrientation(ctx, afterID, thumbnailOrientationBatchSize)
		if err != nil {
			slog.WarnContext(ctx, "thumbnail orientation backfill stopped: failed to list pending files",
				"error", err.Error(), "checked", checked, "remade", remade, "failed", failed)
			return
		}
		if len(batch) == 0 {
			break
		}
		for _, file := range batch {
			afterID = file.ID
			made, err := svc.orientThumbnails(ctx, bucket, reg, file)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				failed++
				slog.DebugContext(ctx, "thumbnail orientation backfill: file skipped", "file_id", file.ID, "error", err.Error())
				continue
			}
			checked++
			if made {
				remade++
			}
		}
	}
	if checked > 0 || failed > 0 {
		slog.InfoContext(ctx, "thumbnail orientation backfill complete",
			"checked", checked, "remade", remade, "failed", failed, "duration_ms", time.Since(start).Milliseconds())
	}
}

// orientThumbnails brings one JPEG file's thumbnails in line with its EXIF
// orientation and marks it. It reports whether it wrote the thumbnails.
func (s *FileService) orientThumbnails(ctx context.Context, bucket *blob.Bucket, reg registry.FileRegistry, file *models.FileEntity) (bool, error) {
	orientation, err := blobJPEGOrientation(ctx, bucket, file.OriginalPath)
	if err != nil {
		return false, err
	}
	if orientation == imageprocessor.OrientationNormal {
		return false, reg.SetThumbnailsOriented(ctx, file.ID)
	}
	has, err := s.hasThumbnails(ctx, file)
	if err != nil || !has {
		return false, err
	}
	return true, s.GenerateThumbnails(ctx, file)
}

// hasThumbnails reports whether any thumbnail of file is stored.
func (s *FileService) hasThumbnails(ctx context.Context, file *models.FileEntity) (bool, error) {
	for size := range s.thumbnailSizes {
		_, exists, err := s.ResolveThumbnail(ctx, file.TenantID, file.ID, size)
		if err != nil || exists {
			return exists, err
		}
	}
	return false, nil
}

// blobJPEGOrientation reads the EXIF orientation from the start of the JPEG
// stored under key.
func blobJPEGOrientation(ctx context.Context, bucket *blob.Bucket, key string) (imageprocessor.Orientation, error) {
	r, err := bucket.NewRangeReader(ctx, key, 0, exifHeadBytes, nil)
	if err != nil {
		return imageprocessor.OrientationNormal, errxtrace.Wrap("failed to open blob", err)
	}
	defer r.Close()
	head, err := io.ReadAll(r)
	if err != nil {
		return imageprocessor.OrientationNormal, errxtrace.Wrap("failed to read blob", err)
	}
	return imageprocessor.JPEGOrientation(head), nil
}
