package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// ---- AI that can look at pictures ----
//
// Pocket AI takes pictures with a prompt (an "images" list next to the
// prompt, Pocket AI 0.87 and newer); an OpenAI-compatible API takes them
// as image parts of the message. Text-only questions go through the
// shared ai.Client.

type aiImage struct {
	Mime string `json:"mime"`
	Data []byte `json:"-"`
}

// aiAsk runs one question, with pictures when there are any, and decodes
// the JSON answer into v.
func (s *Server) aiAsk(ctx context.Context, uid int64, st store.Settings, system, prompt string, images []aiImage, v any) error {
	cl, used, why := s.aiFor(ctx, uid, st)
	if cl == nil {
		return &userErr{firstNonEmpty(why, "AI is not set up (Settings → AI)")}
	}
	if len(images) == 0 {
		return cl.JSON(ctx, system, prompt, v)
	}
	if !st.AIPhotos {
		// the person switched pictures off for the AI
		return &userErr{"sending photos to the AI is switched off (Settings → AI)"}
	}
	var text string
	var err error
	if used == "pocket" {
		text, err = s.pocketVision(ctx, uid, st, system, prompt, images)
	} else {
		text, err = s.openaiVision(ctx, st, system, prompt, images)
	}
	if err != nil {
		return err
	}
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(text), "```json"), "```"), "```"))
	if i := strings.IndexByte(text, '{'); i > 0 {
		text = text[i:]
	}
	if err := json.Unmarshal([]byte(text), v); err != nil {
		return fmt.Errorf("the AI's answer was not valid JSON: %w", err)
	}
	return nil
}

type pocketImage struct {
	Mime string `json:"mime"`
	Data string `json:"data"` // base64
}

func (s *Server) pocketVision(ctx context.Context, uid int64, st store.Settings, system, prompt string, images []aiImage) (string, error) {
	u, err := s.store.GetUser(ctx, uid)
	if err != nil {
		return "", err
	}
	tok, err := s.core.ActAs(ctx, u.Email, "ai", core.InferenceScope)
	if err != nil {
		return "", fmt.Errorf("Pocket AI: %w", err)
	}
	body := map[string]any{"system": system, "prompt": prompt, "json": true}
	if st.AIPocketModel != "" {
		body["model"] = st.AIPocketModel
	}
	var imgs []pocketImage
	for _, im := range images {
		imgs = append(imgs, pocketImage{Mime: im.Mime, Data: base64.StdEncoding.EncodeToString(im.Data)})
	}
	body["images"] = imgs
	raw, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(ctx, 170*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(tok.URL, "/")+"/api/v1/inference", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 170 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("Pocket AI: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&e)
		if res.StatusCode == http.StatusBadRequest && strings.Contains(e.Error, "provide a prompt") {
			return "", &userErr{"this Pocket AI cannot look at photos yet: update Pocket AI, or use your own OpenAI key for photos (Settings → AI)"}
		}
		return "", fmt.Errorf("Pocket AI HTTP %d: %s", res.StatusCode, e.Error)
	}
	var out core.InferenceReply
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Text) == "" {
		return "", errors.New("Pocket AI returned no answer")
	}
	return out.Text, nil
}

func (s *Server) openaiVision(ctx context.Context, st store.Settings, system, prompt string, images []aiImage) (string, error) {
	base := strings.TrimRight(firstNonEmpty(st.OpenAIBaseURL, "https://api.openai.com/v1"), "/")
	model := firstNonEmpty(st.OpenAIModel, "gpt-4o-mini")
	parts := []map[string]any{{"type": "text", "text": prompt}}
	for _, im := range images {
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + im.Mime + ";base64," + base64.StdEncoding.EncodeToString(im.Data), "detail": "low"}})
	}
	body := map[string]any{"model": model, "response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]any{{"role": "system", "content": system}, {"role": "user", "content": parts}}}
	raw, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+st.OpenAIKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 150 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(b, &e)
		return "", fmt.Errorf("AI HTTP %d: %s", res.StatusCode, e.Error.Message)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &out); err != nil || len(out.Choices) == 0 {
		return "", errors.New("the AI returned no answer")
	}
	return out.Choices[0].Message.Content, nil
}

// photoForAI loads one of the person's pictures for the AI (the big one).
func (s *Server) photoForAI(ctx context.Context, uid, id int64) ([]aiImage, error) {
	if id == 0 {
		return nil, nil
	}
	p, err := s.store.GetPhoto(ctx, uid, id, false)
	if err != nil {
		return nil, err
	}
	return []aiImage{{Mime: p.Mime, Data: p.Data}}, nil
}
