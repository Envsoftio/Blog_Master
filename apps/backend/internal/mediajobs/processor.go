package mediajobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"seoblog/apps/backend/internal/media"
	"seoblog/apps/backend/internal/store"
)

type Storage interface {
	GetObject(ctx context.Context, key string, maxBytes int64) ([]byte, string, error)
	PutObject(ctx context.Context, key string, body []byte, contentType string) error
	DeleteObject(ctx context.Context, key string) error
	Bucket() string
}

type Processor struct {
	Store   *store.Store
	Storage Storage
	Logger  *slog.Logger
}

func (p Processor) Process(ctx context.Context, limit int) (int, error) {
	if p.Store == nil || p.Storage == nil {
		return 0, nil
	}
	assets, err := p.Store.ListMediaAssetsForProcessing(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	var errs []error
	for _, asset := range assets {
		if err := p.ProcessAsset(ctx, asset); err != nil {
			errs = append(errs, err)
			p.logError("media asset processing failed", asset, err)
			continue
		}
		processed++
	}
	if _, err := p.CleanupReadyOriginals(ctx, limit); err != nil {
		errs = append(errs, err)
	}
	return processed, errors.Join(errs...)
}

func (p Processor) ProcessAsset(ctx context.Context, asset store.AdminMediaAsset) error {
	if p.Store == nil || p.Storage == nil {
		return nil
	}
	body, _, err := p.Storage.GetObject(ctx, asset.ObjectKey, asset.Bytes)
	if err != nil {
		p.cleanupAssetObjects(ctx, asset, nil)
		_, _ = p.Store.FailMediaAssetSystem(ctx, asset.ProjectID, asset.ID, "could not read uploaded media from B2")
		return fmt.Errorf("read original %s: %w", asset.ID, err)
	}
	if int64(len(body)) != asset.Bytes {
		p.cleanupAssetObjects(ctx, asset, nil)
		_, _ = p.Store.RejectMediaAssetSystem(ctx, asset.ProjectID, asset.ID, "uploaded media byte size did not match the registered size")
		return fmt.Errorf("media %s byte size mismatch", asset.ID)
	}
	processed, err := media.ProcessUpload(asset.Filename, asset.ContentType, body, media.Options{MaxBytes: asset.Bytes})
	if err != nil {
		p.cleanupAssetObjects(ctx, asset, nil)
		if errors.Is(err, media.ErrUnsafeUpload) {
			_, _ = p.Store.RejectMediaAssetSystem(ctx, asset.ProjectID, asset.ID, err.Error())
			return err
		}
		_, _ = p.Store.FailMediaAssetSystem(ctx, asset.ProjectID, asset.ID, "could not process uploaded media")
		return err
	}
	if asset.ExpectedSHA256 != "" && !strings.EqualFold(asset.ExpectedSHA256, processed.SHA256) {
		p.cleanupAssetObjects(ctx, asset, nil)
		_, _ = p.Store.RejectMediaAssetSystem(ctx, asset.ProjectID, asset.ID, "uploaded media checksum did not match the client checksum")
		return fmt.Errorf("media %s checksum mismatch", asset.ID)
	}
	var uploadedKeys []string
	var variantInputs []store.MediaVariantInput
	for _, variant := range processed.Variants {
		objectKey := media.VariantObjectKey(asset.ProjectID, asset.ID, variant.Name)
		if err := p.Storage.PutObject(ctx, objectKey, variant.Bytes, variant.ContentType); err != nil {
			p.cleanupAssetObjects(ctx, asset, uploadedKeys)
			_, _ = p.Store.FailMediaAssetSystem(ctx, asset.ProjectID, asset.ID, "could not upload media variant")
			return fmt.Errorf("upload variant %s for %s: %w", variant.Name, asset.ID, err)
		}
		uploadedKeys = append(uploadedKeys, objectKey)
		variantInputs = append(variantInputs, store.MediaVariantInput{
			Name:        variant.Name,
			ObjectKey:   objectKey,
			ContentType: variant.ContentType,
			Width:       int64(variant.Width),
			Height:      int64(variant.Height),
			Bytes:       int64(len(variant.Bytes)),
		})
	}
	completedObjectKey := ""
	if len(variantInputs) == 0 {
		completedObjectKey = media.ProcessedOriginalObjectKey(asset.ProjectID, asset.ID, asset.Filename)
		if err := p.Storage.PutObject(ctx, completedObjectKey, body, processed.ContentType); err != nil {
			p.cleanupAssetObjects(ctx, asset, uploadedKeys)
			_, _ = p.Store.FailMediaAssetSystem(ctx, asset.ProjectID, asset.ID, "could not move processed media out of pending storage")
			return fmt.Errorf("move processed original %s: %w", asset.ID, err)
		}
		uploadedKeys = append(uploadedKeys, completedObjectKey)
	}
	if _, err := p.Store.CompleteMediaAssetSystem(ctx, asset.ProjectID, asset.ID, store.MediaCompletionInput{
		ObjectKey:   completedObjectKey,
		Bucket:      p.Storage.Bucket(),
		ContentType: processed.ContentType,
		SHA256:      processed.SHA256,
		Width:       int64(processed.Width),
		Height:      int64(processed.Height),
		Metadata:    processed.Metadata,
		Variants:    variantInputs,
	}); err != nil {
		p.deleteAssetKeys(ctx, asset, uploadedKeys)
		return err
	}
	if len(variantInputs) == 0 {
		if err := p.deleteAssetKeys(ctx, asset, []string{asset.ObjectKey}); err != nil {
			if _, restoreErr := p.Store.SetMediaAssetObjectKeySystem(ctx, asset.ProjectID, asset.ID, asset.ObjectKey); restoreErr != nil {
				return errors.Join(
					fmt.Errorf("delete processed original %s: %w", asset.ID, err),
					fmt.Errorf("restore pending media pointer %s: %w", asset.ID, restoreErr),
				)
			}
			return fmt.Errorf("delete processed original %s: %w", asset.ID, err)
		}
		return nil
	}
	if len(variantInputs) > 0 {
		if err := p.deleteAssetKeys(ctx, asset, []string{asset.ObjectKey}); err != nil {
			return fmt.Errorf("delete processed original %s: %w", asset.ID, err)
		}
		if _, err := p.Store.SetMediaAssetObjectKeySystem(ctx, asset.ProjectID, asset.ID, variantInputs[0].ObjectKey); err != nil {
			return fmt.Errorf("promote media variant %s: %w", asset.ID, err)
		}
	}
	return nil
}

func (p Processor) cleanupAssetObjects(ctx context.Context, asset store.AdminMediaAsset, extraKeys []string) {
	keys := []string{asset.ObjectKey}
	for _, variant := range asset.Variants {
		keys = append(keys, variant.ObjectKey)
	}
	keys = append(keys, extraKeys...)
	_ = p.deleteAssetKeys(ctx, asset, keys)
}

func (p Processor) CleanupReadyOriginals(ctx context.Context, limit int) (int, error) {
	if p.Store == nil || p.Storage == nil {
		return 0, nil
	}
	assets, err := p.Store.ListReadyMediaAssetsWithPendingOriginals(ctx, limit)
	if err != nil {
		return 0, err
	}
	cleaned := 0
	var errs []error
	for _, asset := range assets {
		if len(asset.Variants) == 0 {
			promotedObjectKey, err := p.copyReadyPendingOriginal(ctx, asset)
			if err != nil {
				errs = append(errs, fmt.Errorf("move ready original %s: %w", asset.ID, err))
				p.logError("ready media original move failed", asset, err)
				continue
			}
			if _, err := p.Store.SetMediaAssetObjectKeySystem(ctx, asset.ProjectID, asset.ID, promotedObjectKey); err != nil {
				errs = append(errs, fmt.Errorf("promote ready media original %s: %w", asset.ID, err))
				p.logError("ready media original promotion failed", asset, err)
				continue
			}
			if err := p.deleteAssetKeys(ctx, asset, []string{asset.ObjectKey}); err != nil {
				if _, restoreErr := p.Store.SetMediaAssetObjectKeySystem(ctx, asset.ProjectID, asset.ID, asset.ObjectKey); restoreErr != nil {
					err = errors.Join(err, fmt.Errorf("restore pending media pointer: %w", restoreErr))
				}
				errs = append(errs, fmt.Errorf("delete ready original %s: %w", asset.ID, err))
				p.logError("ready media original cleanup failed", asset, err)
				continue
			}
			cleaned++
			continue
		}
		promotedObjectKey := strings.TrimSpace(asset.Variants[0].ObjectKey)
		if promotedObjectKey == "" || !media.DeletableObjectKeyForAsset(asset.ProjectID, asset.ID, promotedObjectKey) {
			err := fmt.Errorf("refusing to promote media asset %s to unsafe object key %q", asset.ID, promotedObjectKey)
			errs = append(errs, err)
			p.logError("ready media original cleanup failed", asset, err)
			continue
		}
		if err := p.deleteAssetKeys(ctx, asset, []string{asset.ObjectKey}); err != nil {
			errs = append(errs, fmt.Errorf("delete ready original %s: %w", asset.ID, err))
			p.logError("ready media original cleanup failed", asset, err)
			continue
		}
		if _, err := p.Store.SetMediaAssetObjectKeySystem(ctx, asset.ProjectID, asset.ID, promotedObjectKey); err != nil {
			errs = append(errs, fmt.Errorf("promote ready media original %s: %w", asset.ID, err))
			p.logError("ready media original promotion failed", asset, err)
			continue
		}
		cleaned++
	}
	return cleaned, errors.Join(errs...)
}

func (p Processor) copyReadyPendingOriginal(ctx context.Context, asset store.AdminMediaAsset) (string, error) {
	body, contentType, err := p.Storage.GetObject(ctx, asset.ObjectKey, asset.Bytes)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = asset.ContentType
	}
	objectKey := media.ProcessedOriginalObjectKey(asset.ProjectID, asset.ID, asset.Filename)
	if err := p.Storage.PutObject(ctx, objectKey, body, contentType); err != nil {
		return "", err
	}
	return objectKey, nil
}

func (p Processor) deleteAssetKeys(ctx context.Context, asset store.AdminMediaAsset, keys []string) error {
	var errs []error
	for _, key := range uniqueKeys(keys) {
		if !media.DeletableObjectKeyForAsset(asset.ProjectID, asset.ID, key) {
			err := fmt.Errorf("unsafe media object key")
			errs = append(errs, err)
			p.logError("refusing to delete media object outside asset scope", store.AdminMediaAsset{
				ID:        asset.ID,
				ProjectID: asset.ProjectID,
				ObjectKey: key,
			}, err)
			continue
		}
		if err := p.Storage.DeleteObject(ctx, key); err != nil {
			errs = append(errs, err)
			p.logError("media object cleanup failed", store.AdminMediaAsset{
				ID:        asset.ID,
				ProjectID: asset.ProjectID,
				ObjectKey: key,
			}, err)
		}
	}
	return errors.Join(errs...)
}

func uniqueKeys(keys []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if key = strings.TrimSpace(key); key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func (p Processor) logError(message string, asset store.AdminMediaAsset, err error) {
	if p.Logger == nil || err == nil {
		return
	}
	p.Logger.Error(message, "asset_id", asset.ID, "object_key", asset.ObjectKey, "error", err)
}
