package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"strings"
)

func extractArt(trackPath string) (string, error) {
	tmp, err := os.CreateTemp("", "musicplayer-art-*.jpg")
	if err != nil {
		return "", err
	}
	tmp.Close()

	err = exec.Command("ffmpeg",
		"-y",
		"-i", trackPath,
		"-map", "0:v:0",
		"-vframes", "1",
		"-vf", "scale=500:500:force_original_aspect_ratio=decrease,pad=500:500:(ow-iw)/2:(oh-ih)/2",
		tmp.Name(),
	).Run()
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func imageToHalfBlock(img image.Image, width, height int) string {

	pixH := height * 2
	pixW := width

	bounds := img.Bounds()
	iw := bounds.Max.X - bounds.Min.X
	ih := bounds.Max.Y - bounds.Min.Y

	var sb strings.Builder

	for row := 0; row < pixH; row += 2 {
		for col := 0; col < pixW; col++ {

			tx := bounds.Min.X + col*iw/pixW
			ty := bounds.Min.Y + row*ih/pixH

			bx := tx
			by := bounds.Min.Y + (row+1)*ih/pixH

			tr, tg, tb, _ := img.At(tx, ty).RGBA()
			br, bg, bb, _ := img.At(bx, by).RGBA()

			topR, topG, topB := tr>>8, tg>>8, tb>>8
			botR, botG, botB := br>>8, bg>>8, bb>>8

			sb.WriteString(fmt.Sprintf(
				"\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				topR, topG, topB,
				botR, botG, botB,
			))
		}
		sb.WriteString("\x1b[0m\n")
	}

	return sb.String()
}

func renderArt(trackPath string, maxW, maxH int) string {
	artFile, err := extractArt(trackPath)
	if err != nil {
		return noArtPlaceholder(maxW, maxH)
	}
	defer os.Remove(artFile)

	f, err := os.Open(artFile)
	if err != nil {
		return noArtPlaceholder(maxW, maxH)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return noArtPlaceholder(maxW, maxH)
	}

	return imageToHalfBlock(img, maxW, maxH)
}

func noArtPlaceholder(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	line := "┌" + strings.Repeat("─", w-2) + "┐\n"
	mid := "│" + strings.Repeat(" ", w-2) + "│\n"
	bot := "└" + strings.Repeat("─", w-2) + "┘\n"

	var sb strings.Builder
	sb.WriteString(line)
	for i := 1; i < h-1; i++ {
		if i == h/2 {
			label := "  NO ART  "
			pad := (w - 2 - len(label)) / 2
			sb.WriteString("│" + strings.Repeat(" ", pad) + label + strings.Repeat(" ", w-2-pad-len(label)) + "│\n")
		} else {
			sb.WriteString(mid)
		}
	}
	sb.WriteString(bot)
	return sb.String()
}