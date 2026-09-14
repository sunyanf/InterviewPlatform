package resumeparse

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ExtractText 从简历文件中提取纯文本
// 支持 .txt、.pdf、.docx
func ExtractText(fileName string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".txt", ".md":
		return string(data), nil
	case ".pdf":
		return extractPDF(data)
	case ".docx":
		return extractDOCX(data)
	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}

// extractPDF 从 PDF 中提取文本
func extractPDF(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}

	var textBuilder strings.Builder
	totalPage := reader.NumPage()
	for i := 1; i <= totalPage; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		pageText, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		textBuilder.WriteString(pageText)
		textBuilder.WriteString("\n")
	}

	return textBuilder.String(), nil
}

// extractDOCX 从 DOCX 中提取文本
// DOCX 本质是 ZIP，内容在 word/document.xml 中
func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}

	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open document.xml: %w", err)
		}
		defer rc.Close()

		content, err := io.ReadAll(rc)
		if err != nil {
			return "", fmt.Errorf("read document.xml: %w", err)
		}

		return extractTextFromDOCXXML(content), nil
	}

	return "", fmt.Errorf("document.xml not found in docx")
}

// docxDocument DOCX XML 结构
type docxDocument struct {
	Body docxBody `xml:"body"`
}

type docxBody struct {
	Paragraphs []docxParagraph `xml:"p"`
}

type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}

type docxRun struct {
	Text string `xml:"t"`
}

// extractTextFromDOCXXML 从 DOCX XML 中提取文本
func extractTextFromDOCXXML(data []byte) string {
	var doc docxDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return ""
	}

	var builder strings.Builder
	for _, p := range doc.Body.Paragraphs {
		for _, r := range p.Runs {
			builder.WriteString(r.Text)
		}
		builder.WriteString("\n")
	}
	return builder.String()
}
