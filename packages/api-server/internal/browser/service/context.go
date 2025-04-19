package service

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/playwright-community/playwright-go"

	model "github.com/babelcloud/gbox/packages/api-server/pkg/browser"
)

// findManagedContext locates a ManagedContext by its ID by iterating.
func (s *BrowserService) findManagedContext(contextID string) (*ManagedContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, mb := range s.managedBrowsers {
		mb.mu.RLock()
		mc, exists := mb.Contexts[contextID]
		mb.mu.RUnlock()
		if exists {
			return mc, nil
		}
	}
	return nil, ErrContextNotFound
}

// CreateContext creates a new browser context within a ManagedBrowser.
func (s *BrowserService) CreateContext(boxID string, params model.CreateContextParams) (*model.CreateContextResult, error) {
	mb, err := s.getOrCreateManagedBrowser(boxID)
	if err != nil {
		return nil, err
	}

	opts := playwright.BrowserNewContextOptions{
		UserAgent:   playwright.String(params.UserAgent),
		Locale:      playwright.String(params.Locale),
		TimezoneId:  playwright.String(params.Timezone),
		Permissions: params.Permissions,
	}
	if params.ViewportWidth > 0 && params.ViewportHeight > 0 {
		opts.Viewport = &playwright.Size{
			Width:  params.ViewportWidth,
			Height: params.ViewportHeight,
		}
	}

	newContextInstance, err := mb.Instance.NewContext(opts)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "socket hang up") || strings.Contains(errMsg, "connect") || strings.Contains(errMsg, "disconnected") {
			fmt.Printf("WARN: Connection error during NewContext for box %s, cleaning up browser entry: %v\n", boxID, err)
			s.handleBrowserDisconnect(boxID)
		}
		return nil, fmt.Errorf("failed to create Playwright context for box %s: %w", boxID, err)
	}

	contextID := uuid.New().String()
	mc := &ManagedContext{
		ID:            contextID,
		Instance:      newContextInstance,
		ParentBrowser: mb,
		Pages:         make(map[string]*ManagedPage),
	}

	mb.mu.Lock()
	mb.Contexts[contextID] = mc
	mb.mu.Unlock()

	newContextInstance.Once("close", func() {
		fmt.Printf("INFO: Context %s close event\n", contextID)
		s.handleContextClose(mc)
	})

	result := model.NewCreateContextResult(contextID)
	return &result, nil
}

// cleanupManagedContext_locked removes pages associated with a context from the global map.
// Assumes s.mu is locked.
func (s *BrowserService) cleanupManagedContext_locked(mc *ManagedContext) {
	if mc == nil {
		return
	}
	mc.mu.Lock() // Lock the context's page map
	defer mc.mu.Unlock()

	fmt.Printf("INFO: Cleaning up %d pages for closed context %s\n", len(mc.Pages), mc.ID)
	for pageID := range mc.Pages {
		delete(s.pageMap, pageID) // Remove page from global index
	}
	mc.Pages = make(map[string]*ManagedPage) // Reset context's page map
}

// handleContextClose removes the ManagedContext from its parent and cleans page resources.
func (s *BrowserService) handleContextClose(mc *ManagedContext) {
	if mc == nil || mc.ParentBrowser == nil {
		fmt.Printf("WARN: handleContextClose called with nil context or parent\n")
		return
	}

	s.mu.Lock()                        // Lock global service maps
	s.cleanupManagedContext_locked(mc) // Clean pages from global map
	s.mu.Unlock()

	// Remove context from parent browser's map
	parent := mc.ParentBrowser
	parent.mu.Lock()
	delete(parent.Contexts, mc.ID)
	parent.mu.Unlock()
	fmt.Printf("INFO: Removed context %s from managed browser for box %s\n", mc.ID, parent.BoxID)
}

// CloseContext closes a specific browser context.
func (s *BrowserService) CloseContext(boxID, contextID string) error {
	mc, err := s.findManagedContext(contextID)
	if err != nil {
		return err
	}
	if mc.ParentBrowser.BoxID != boxID {
		return fmt.Errorf("context %s does not belong to box %s", contextID, boxID)
	}
	if err := mc.Instance.Close(); err != nil {
		fmt.Printf("WARN: Failed to explicitly close context %s: %v\n", contextID, err)
		s.handleContextClose(mc) // Force map cleanup
		return fmt.Errorf("failed to close context %s: %w", contextID, err)
	}
	return nil
}
