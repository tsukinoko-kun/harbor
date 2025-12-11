package docker

import (
	"context"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/image"
)

// Image represents a Docker image with relevant information.
type Image struct {
	ID      string
	Tags    []string
	Size    int64
	Created int64
}

// ListImages returns all images.
func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	images, err := c.cli.ImageList(ctx, image.ListOptions{
		All: false, // Don't include intermediate images
	})
	if err != nil {
		return nil, err
	}

	result := make([]Image, 0, len(images))
	for _, img := range images {
		id := img.ID
		if strings.HasPrefix(id, "sha256:") {
			id = id[7:19] // Take first 12 chars after sha256:
		}

		tags := img.RepoTags
		if len(tags) == 0 {
			tags = []string{"<none>:<none>"}
		}

		result = append(result, Image{
			ID:      id,
			Tags:    tags,
			Size:    img.Size,
			Created: img.Created,
		})
	}

	// Sort by first tag name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Tags[0] < result[j].Tags[0]
	})

	return result, nil
}

// FormatSize formats a size in bytes to a human-readable string.
func FormatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return formatFloat(float64(size)/GB) + " GB"
	case size >= MB:
		return formatFloat(float64(size)/MB) + " MB"
	case size >= KB:
		return formatFloat(float64(size)/KB) + " KB"
	default:
		return formatInt(size) + " B"
	}
}

func formatFloat(f float64) string {
	if f >= 100 {
		return formatInt(int64(f))
	}
	if f >= 10 {
		return strings.TrimSuffix(strings.TrimSuffix(
			formatFloatPrec(f, 1), "0"), ".")
	}
	return strings.TrimSuffix(strings.TrimSuffix(
		formatFloatPrec(f, 2), "0"), "0")
}

func formatFloatPrec(f float64, prec int) string {
	intPart := int64(f)
	fracPart := f - float64(intPart)
	if fracPart < 0 {
		fracPart = -fracPart
	}

	result := formatInt(intPart) + "."
	for i := 0; i < prec; i++ {
		fracPart *= 10
		digit := int64(fracPart)
		result += formatInt(digit)
		fracPart -= float64(digit)
	}
	return result
}

func formatInt(i int64) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
