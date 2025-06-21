package beatport

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

// isPageLoaded checks if the page is fully loaded
func isPageLoaded(ctx context.Context) bool {
	var readyState string
	err := chromedp.Evaluate(`document.readyState`, &readyState).Do(ctx)
	return err == nil && readyState == "complete"
}

// ChromeDriver manages the Chrome browser instance for web automation
type ChromeDriver struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// isContextValid checks if the chromedp context is still valid
func (cd *ChromeDriver) isContextValid() error {
	select {
	case <-cd.ctx.Done():
		return cd.ctx.Err()
	default:
		return nil
	}
}

// InitializeChrome initializes a new Chrome browser instance with chromedp
func InitializeChrome(headless bool) (*ChromeDriver, error) {
	// Configure Chrome options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-timer-throttling", false),
		chromedp.Flag("disable-backgrounding-occluded-windows", false),
		chromedp.Flag("disable-renderer-backgrounding", false),
		chromedp.Flag("disable-features", "TranslateUI"),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36"),
	)

	// Create context with Chrome options
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	// Create browser context with extended timeout for long-running operations like pagination
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))

	// Add a longer timeout specifically for the ChromeDriver context to prevent premature cancellation
	// This is important for pagination which can take several minutes
	ctx, contextCancel := context.WithTimeout(ctx, 10*time.Minute)
	combinedCancel := func() {
		contextCancel()
		cancel()
		allocCancel()
	}

	// Start the browser
	if err := chromedp.Run(ctx); err != nil {
		combinedCancel()
		return nil, fmt.Errorf("failed to start Chrome: %w", err)
	}

	return &ChromeDriver{
		ctx:    ctx,
		cancel: combinedCancel,
	}, nil
}

// Close shuts down the Chrome browser instance
func (cd *ChromeDriver) Close() {
	if cd.cancel != nil {
		cd.cancel()
	}
}

// LoginToBeatport logs into Beatport using the provided credentials
func (cd *ChromeDriver) LoginToBeatport(username, password string) error {
	log.Printf("🔐 [LOGIN BREAKPOINT 1] Starting login process for user: %s", username)
	if username == "" || password == "" {
		return errors.New("username and password are required")
	}

	return chromedp.Run(cd.ctx,
		// Navigate to Beatport homepage first
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 2] Navigating to Beatport homepage: %s", BeatportMainUrl)
			return chromedp.Navigate(BeatportMainUrl).Do(ctx)
		}),

		// Wait for the page to load
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 3] Waiting for homepage to load...")
			return chromedp.WaitReady(`body`, chromedp.ByQuery).Do(ctx)
		}),

		// Wait a bit for any dynamic content to load
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 4] Waiting 2 seconds for dynamic content...")
			return chromedp.Sleep(2 * time.Second).Do(ctx)
		}),

		// Look for and click the login button/link
		// Try multiple possible selectors for the login button
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 5] Looking for login button on homepage...")

			// Check if page is fully loaded to determine timeout strategy
			pageLoaded := isPageLoaded(ctx)
			selectorTimeout := 2 * time.Second
			if pageLoaded {
				log.Printf("🔐 [LOGIN BREAKPOINT 5.0] Page fully loaded, using fast selector checks")
				selectorTimeout = 100 * time.Millisecond // Very short timeout when page is ready
			} else {
				log.Printf("🔐 [LOGIN BREAKPOINT 5.0] Page still loading, using longer timeouts")
			}

			// Try different possible selectors for the login button (based on actual HTML structure)
			selectors := []string{
				`li.header_item button:has(span:contains("Login"))`, // Button in header_item containing Login span
				`button:has([data-testid="icon-person"])`,           // Button containing person icon
				`li.header_item button`,                             // Any button in header item
				`button:has(span:contains("Login"))`,                // Button containing Login span
				`button:contains("Login")`,                          // Button with "Login" text
				`a[href*="login"]`,                                  // Link containing "login" in href
				`a:contains("Login")`,                               // Link with "Login" text
				`[data-testid="login"]`,                             // Element with login test id
				`.login-button`,                                     // Element with login class
				`#login-button`,                                     // Element with login id
			}

			for i, selector := range selectors {
				log.Printf("🔐 [LOGIN BREAKPOINT 5.%d] Trying selector: %s", i+1, selector)

				// Create a timeout context for each selector attempt
				timeoutCtx, cancel := context.WithTimeout(ctx, selectorTimeout)
				var nodes []*cdp.Node
				err := chromedp.Nodes(selector, &nodes, chromedp.ByQuery).Do(timeoutCtx)
				cancel() // Always cancel to free resources

				if err == nil && len(nodes) > 0 {
					log.Printf("🔐 [LOGIN BREAKPOINT 6] Found login button with selector: %s, clicking...", selector)
					return chromedp.Click(selector, chromedp.ByQuery).Do(ctx)
				} else if err != nil {
					log.Printf("🔐 [LOGIN BREAKPOINT 5.%d] Selector failed: %v", i+1, err)
				} else {
					log.Printf("🔐 [LOGIN BREAKPOINT 5.%d] Selector found no elements", i+1)
				}
			}
			log.Printf("🔐 [LOGIN BREAKPOINT ERROR] Could not find login button on homepage")
			return fmt.Errorf("could not find login button on homepage")
		}),

		// Wait for redirect to account.beatport.com
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 7] Waiting for redirect to login page...")
			// Wait for URL to change
			for i := 0; i < 30; i++ { // Wait up to 30 seconds
				var url string
				if err := chromedp.Location(&url).Do(ctx); err != nil {
					return err
				}
				if url != BeatportMainUrl && url != BeatportMainUrl+"/" {
					log.Printf("🔐 [LOGIN BREAKPOINT 8] Successfully redirected to: %s", url)
					return nil
				}
				time.Sleep(1 * time.Second)
			}
			return fmt.Errorf("timeout waiting for redirect")
		}),

		// Wait for the login form to appear
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 9] Waiting for login form to appear...")
			return chromedp.WaitVisible(`form`, chromedp.ByQuery).Do(ctx)
		}),

		// Wait a bit more for any dynamic content to load
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 10] Waiting 2 seconds for form to fully load...")
			return chromedp.Sleep(2 * time.Second).Do(ctx)
		}),

		// Find and fill the username field
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 11] Looking for username field...")

			// Check if page is fully loaded to determine timeout strategy
			pageLoaded := isPageLoaded(ctx)
			selectorTimeout := 2 * time.Second
			if pageLoaded {
				log.Printf("🔐 [LOGIN BREAKPOINT 11.0] Page fully loaded, using fast selector checks")
				selectorTimeout = 100 * time.Millisecond // Very short timeout when page is ready
			} else {
				log.Printf("🔐 [LOGIN BREAKPOINT 11.0] Page still loading, using longer timeouts")
			}

			// Try different possible selectors for username field
			usernameSelectors := []string{
				`#username`,      // Most specific - id="username"
				`input#username`, // Input with id="username"
				`input.MuiInputBase-input[id="username"]`,        // Material-UI specific
				`input[aria-describedby="username-helper-text"]`, // Material-UI aria attribute
				`input[name="username"]`,
				`input[name="email"]`,
				`input[type="email"]`,
				`input[placeholder*="username" i]`,
				`input[placeholder*="email" i]`,
				`#email`,
			}

			for i, selector := range usernameSelectors {
				log.Printf("🔐 [LOGIN BREAKPOINT 11.%d] Trying username selector: %s", i+1, selector)

				// Create a timeout context for each selector attempt
				timeoutCtx, cancel := context.WithTimeout(ctx, selectorTimeout)
				var nodes []*cdp.Node
				err := chromedp.Nodes(selector, &nodes, chromedp.ByQuery).Do(timeoutCtx)
				cancel() // Always cancel to free resources

				if err == nil && len(nodes) > 0 {
					log.Printf("🔐 [LOGIN BREAKPOINT 12] Found username field with selector: %s, filling...", selector)
					if err := chromedp.Clear(selector, chromedp.ByQuery).Do(ctx); err != nil {
						return err
					}
					return chromedp.SendKeys(selector, username, chromedp.ByQuery).Do(ctx)
				} else if err != nil {
					log.Printf("🔐 [LOGIN BREAKPOINT 11.%d] Selector failed: %v", i+1, err)
				} else {
					log.Printf("🔐 [LOGIN BREAKPOINT 11.%d] Selector found no elements", i+1)
				}
			}
			log.Printf("🔐 [LOGIN BREAKPOINT ERROR] Could not find username field")
			return fmt.Errorf("could not find username field")
		}),

		// Find and fill the password field
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 13] Looking for password field...")

			// Check if page is fully loaded to determine timeout strategy
			pageLoaded := isPageLoaded(ctx)
			selectorTimeout := 2 * time.Second
			if pageLoaded {
				log.Printf("🔐 [LOGIN BREAKPOINT 13.0] Page fully loaded, using fast selector checks")
				selectorTimeout = 100 * time.Millisecond // Very short timeout when page is ready
			} else {
				log.Printf("🔐 [LOGIN BREAKPOINT 13.0] Page still loading, using longer timeouts")
			}

			// Try different possible selectors for password field
			passwordSelectors := []string{
				`#password`,      // Most specific - id="password"
				`input#password`, // Input with id="password"
				`input.MuiInputBase-input[id="password"]`,        // Material-UI specific
				`input[aria-describedby="password-helper-text"]`, // Material-UI aria attribute
				`input[type="password"]`,
				`input[name="password"]`,
				`input[placeholder*="password" i]`,
			}

			for i, selector := range passwordSelectors {
				log.Printf("🔐 [LOGIN BREAKPOINT 13.%d] Trying password selector: %s", i+1, selector)

				// Create a timeout context for each selector attempt
				timeoutCtx, cancel := context.WithTimeout(ctx, selectorTimeout)
				var nodes []*cdp.Node
				err := chromedp.Nodes(selector, &nodes, chromedp.ByQuery).Do(timeoutCtx)
				cancel() // Always cancel to free resources

				if err == nil && len(nodes) > 0 {
					log.Printf("🔐 [LOGIN BREAKPOINT 14] Found password field with selector: %s, filling...", selector)
					if err := chromedp.Clear(selector, chromedp.ByQuery).Do(ctx); err != nil {
						return err
					}
					return chromedp.SendKeys(selector, password, chromedp.ByQuery).Do(ctx)
				} else if err != nil {
					log.Printf("🔐 [LOGIN BREAKPOINT 13.%d] Selector failed: %v", i+1, err)
				} else {
					log.Printf("🔐 [LOGIN BREAKPOINT 13.%d] Selector found no elements", i+1)
				}
			}
			log.Printf("🔐 [LOGIN BREAKPOINT ERROR] Could not find password field")
			return fmt.Errorf("could not find password field")
		}),

		// Submit the form
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 15] Looking for submit button...")

			// Check if page is fully loaded to determine timeout strategy
			pageLoaded := isPageLoaded(ctx)
			selectorTimeout := 2 * time.Second
			if pageLoaded {
				log.Printf("🔐 [LOGIN BREAKPOINT 15.0] Page fully loaded, using fast selector checks")
				selectorTimeout = 100 * time.Millisecond // Very short timeout when page is ready
			} else {
				log.Printf("🔐 [LOGIN BREAKPOINT 15.0] Page still loading, using longer timeouts")
			}

			// Try different possible selectors for submit button (based on actual HTML structure)
			submitSelectors := []string{
				`button.MuiButton-root[type="submit"]:has(span:contains("Log In"))`, // Material-UI button with Log In span
				`button.MuiButtonBase-root[type="submit"]`,                          // Material-UI button base
				`button.MuiButton-contained[type="submit"]`,                         // Material-UI contained button
				`button:has(span:contains("Log In"))`,                               // Button containing Log In span
				`button[type="submit"]`,                                             // Standard submit button
				`input[type="submit"]`,                                              // Submit input
				`button:contains("Login")`,                                          // Button with "Login" text
				`button:contains("Sign In")`,                                        // Button with "Sign In" text
				`button:contains("Log In")`,                                         // Button with "Log In" text
				`.submit-button`,                                                    // Element with submit button class
				`#submit-button`,                                                    // Element with submit button id
			}

			for i, selector := range submitSelectors {
				log.Printf("🔐 [LOGIN BREAKPOINT 15.%d] Trying submit selector: %s", i+1, selector)

				// Create a timeout context for each selector attempt
				timeoutCtx, cancel := context.WithTimeout(ctx, selectorTimeout)
				var nodes []*cdp.Node
				err := chromedp.Nodes(selector, &nodes, chromedp.ByQuery).Do(timeoutCtx)
				cancel() // Always cancel to free resources

				if err == nil && len(nodes) > 0 {
					log.Printf("🔐 [LOGIN BREAKPOINT 16] Found submit button with selector: %s, clicking...", selector)
					return chromedp.Click(selector, chromedp.ByQuery).Do(ctx)
				} else if err != nil {
					log.Printf("🔐 [LOGIN BREAKPOINT 15.%d] Selector failed: %v", i+1, err)
				} else {
					log.Printf("🔐 [LOGIN BREAKPOINT 15.%d] Selector found no elements", i+1)
				}
			}
			log.Printf("🔐 [LOGIN BREAKPOINT ERROR] Could not find submit button")
			return fmt.Errorf("could not find submit button")
		}),

		// Wait for login to complete - look for either success or error indicators
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 17] Waiting for login to complete...")
			return chromedp.WaitReady(`body`, chromedp.ByQuery).Do(ctx)
		}),

		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 18] Waiting 3 seconds for page transition...")
			return chromedp.Sleep(3 * time.Second).Do(ctx)
		}),

		// Check if login was successful by looking for user-specific content
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔐 [LOGIN BREAKPOINT 19] Checking login success...")
			var url string
			if err := chromedp.Location(&url).Do(ctx); err != nil {
				return fmt.Errorf("failed to get current URL: %w", err)
			}

			// If we're still on the login page, login likely failed
			if url == BeatportAccountUrl || url == BeatportAccountUrl+"/" {
				log.Printf("🔐 [LOGIN BREAKPOINT ERROR] Login failed - still on login page: %s", url)
				return errors.New("login failed - still on login page")
			}

			log.Printf("🔐 [LOGIN BREAKPOINT 20] Login successful! Redirected to: %s", url)
			return nil
		}),
	)
}

// NavigateToPage navigates to a specific page on Beatport
func (cd *ChromeDriver) NavigateToPage(url string) error {
	return chromedp.Run(cd.ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady(`body`, chromedp.ByQuery),
	)
}

// ExtractAllTrackLinksWithPagination extracts all track links from all pages by clicking "Next" button
func (cd *ChromeDriver) ExtractAllTrackLinksWithPagination(baseUrl string) ([]string, error) {
	log.Printf("🔍 [PAGINATION BREAKPOINT 1] Starting paginated track link extraction from: %s", baseUrl)

	// Navigate to the first page
	if err := cd.NavigateToPage(baseUrl); err != nil {
		return nil, fmt.Errorf("failed to navigate to base URL: %w", err)
	}

	var allTrackLinks []string
	allLinksMap := make(map[string]bool) // To avoid duplicates across pages
	page := 1

	for {
		log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Scraping page %d", page, page)

		// Wait for content to load
		time.Sleep(3 * time.Second)

		// Extract track links from current page
		pageTrackLinks, err := cd.ExtractTrackLinks()
		if err != nil {
			log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Failed to extract links from page %d: %v", page, page, err)
			return nil, fmt.Errorf("failed to extract track links from page %d: %w", page, err)
		}

		log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Found %d track links on page %d", page, len(pageTrackLinks), page)

		// If no track links found, we've reached the end
		if len(pageTrackLinks) == 0 {
			log.Printf("🔍 [PAGINATION BREAKPOINT 3] No track links found on page %d, pagination complete", page)
			break
		}

		// Add unique track links to our collection
		newLinksCount := 0
		for _, link := range pageTrackLinks {
			if !allLinksMap[link] {
				allTrackLinks = append(allTrackLinks, link)
				allLinksMap[link] = true
				newLinksCount++
			}
		}

		log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Added %d new unique track links from page %d (total: %d)", page, newLinksCount, page, len(allTrackLinks))

		// Look for "Next" button and click it
		log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Looking for Next button...", page)

		nextButtonFound := false

		// Check if context is still valid and log its state
		if contextErr := cd.isContextValid(); contextErr != nil {
			log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Context is invalid: %v, stopping pagination", page, contextErr)
			return allTrackLinks, fmt.Errorf("context became invalid: %w", contextErr)
		}
		log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Context is valid, proceeding...", page)

		// First, test if basic operations still work
		var currentURL string
		urlErr := chromedp.Location(&currentURL).Do(cd.ctx)
		if urlErr != nil {
			log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Failed to get current URL (context may be invalid): %v", page, urlErr)
			// If we can't get the URL, the context is definitely invalid
			if urlErr.Error() == "invalid context" ||
				urlErr.Error() == "context canceled" ||
				urlErr.Error() == "context deadline exceeded" {
				log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Context is invalid, stopping pagination", page)
				return allTrackLinks, fmt.Errorf("context invalid, cannot continue pagination: %w", urlErr)
			}
			return allTrackLinks, fmt.Errorf("context error, cannot get URL: %w", urlErr)
		}
		log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Current URL: %s", page, currentURL)

		// Wait for page to be fully loaded before trying to find Next button
		log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Waiting for page to be ready...", page)
		readyErr := chromedp.WaitReady(`body`, chromedp.ByQuery).Do(cd.ctx)
		if readyErr != nil {
			log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Failed to wait for page ready: %v", page, readyErr)
			if readyErr.Error() == "invalid context" ||
				readyErr.Error() == "context canceled" ||
				readyErr.Error() == "context deadline exceeded" {
				log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Context invalid during page ready wait", page)
				return allTrackLinks, fmt.Errorf("context invalid during page ready wait: %w", readyErr)
			}
		} else {
			log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Page is ready", page)
		}

		// Use JavaScript to find Beatport's Next button and get its href
		var jsResult interface{}
		jsCode := `
			// Find Next button using the working approach
			const nextButton = Array.from(document.querySelectorAll('a[href*="page="]'))
							  .find(a => a.innerHTML.includes('Next'));
			
			if (nextButton) {
				return nextButton.href;
			}
			return null;
		`

		log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Executing JavaScript to find Next button...", page)
		evalErr := chromedp.Evaluate(jsCode, &jsResult).Do(cd.ctx)
		if evalErr != nil {
			log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] JavaScript evaluation failed: %v", page, evalErr)
			// Check if this is a context error
			if evalErr.Error() == "invalid context" {
				log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Context became invalid during JavaScript evaluation", page)
				return allTrackLinks, fmt.Errorf("context invalid during JS evaluation: %w", evalErr)
			}
		}

		if evalErr == nil && jsResult != nil {
			if nextHref, ok := jsResult.(string); ok && nextHref != "" {
				log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Found Next button with href: %s", page, nextHref)

				// Use chromedp to click the specific Next link
				selector := fmt.Sprintf(`a[href="%s"]`, nextHref)
				log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Attempting to click with selector: %s", page, selector)

				clickErr := chromedp.Run(cd.ctx,
					chromedp.Click(selector, chromedp.ByQuery, chromedp.NodeVisible),
				)

				if clickErr != nil {
					log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Failed to click Next button: %v", page, clickErr)
					// Check if this is a context error
					if clickErr.Error() == "invalid context" {
						log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Context became invalid during click", page)
						return allTrackLinks, fmt.Errorf("context invalid during click: %w", clickErr)
					}
				} else {
					log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] Successfully clicked Next button", page)
					nextButtonFound = true
					time.Sleep(3 * time.Second) // Wait for page to load
				}
			}
		} else {
			log.Printf("🔍 [PAGINATION BREAKPOINT 2.%d] No Next button found (jsResult: %v)", page, jsResult)
		}

		// If no Next button found, we've reached the end
		if !nextButtonFound {
			log.Printf("🔍 [PAGINATION BREAKPOINT 3] No Next button found on page %d, pagination complete", page)
			break
		}

		// Move to next page
		page++

		// Safety check to prevent infinite loops
		if page > 1000 { // Reasonable upper limit
			log.Printf("🔍 [PAGINATION BREAKPOINT ERROR] Reached maximum page limit (1000), stopping pagination")
			break
		}
	}

	log.Printf("🔍 [PAGINATION BREAKPOINT 4] Pagination complete! Found %d total unique track links across %d pages", len(allTrackLinks), page)
	return allTrackLinks, nil
}

// ExtractTrackLinks extracts all href links that contain "track" from the current page
func (cd *ChromeDriver) ExtractTrackLinks() ([]string, error) {
	log.Printf("🔍 [EXTRACT BREAKPOINT 1] Starting track link extraction...")
	var trackLinks []string

	err := chromedp.Run(cd.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔍 [EXTRACT BREAKPOINT 2] Waiting for tracks to load...")

			// Get current URL for context
			var currentURL string
			if err := chromedp.Location(&currentURL).Do(ctx); err == nil {
				log.Printf("🔍 [EXTRACT BREAKPOINT 3] Current page URL: %s", currentURL)
			}

			// Wait for content to load - try waiting for track containers
			log.Printf("🔍 [EXTRACT BREAKPOINT 4] Waiting for track containers to appear...")
			err := chromedp.WaitReady(`body`, chromedp.ByQuery).Do(ctx)
			if err != nil {
				log.Printf("🔍 [EXTRACT BREAKPOINT 4] Body wait failed: %v", err)
			}

			// Wait a bit more for dynamic content
			time.Sleep(3 * time.Second)
			log.Printf("🔍 [EXTRACT BREAKPOINT 5] Finished waiting, now extracting...")

			// Use JavaScript evaluation to extract track links (same as browser console)
			log.Printf("🔍 [EXTRACT BREAKPOINT 6] Using JavaScript evaluation to find track links...")
			var jsResult interface{}
			jsCode := `Array.from(document.querySelectorAll('a[href*="/track/"]')).map(a => a.href)`

			evalErr := chromedp.Evaluate(jsCode, &jsResult).Do(ctx)
			if evalErr != nil {
				log.Printf("🔍 [EXTRACT BREAKPOINT 6] JavaScript evaluation failed: %v", evalErr)
				log.Printf("🔍 [EXTRACT BREAKPOINT 6] Falling back to DOM node extraction...")

				// Fallback to DOM node approach
				var nodes []*cdp.Node
				timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				fallbackErr := chromedp.Nodes(`a[href*="/track/"]`, &nodes, chromedp.ByQuery).Do(timeoutCtx)
				cancel()

				if fallbackErr != nil {
					log.Printf("🔍 [EXTRACT BREAKPOINT 6] DOM fallback also failed: %v", fallbackErr)
					return fmt.Errorf("failed to extract track links: %w", fallbackErr)
				}

				log.Printf("🔍 [EXTRACT BREAKPOINT 6] DOM fallback found %d elements", len(nodes))
				linkMap := make(map[string]bool)

				for i, node := range nodes {
					href := node.AttributeValue("href")
					if href != "" {
						// Convert relative URLs to absolute URLs
						if href[0] == '/' {
							href = BeatportMainUrl + href
						}

						// Only add if unique
						if !linkMap[href] {
							trackLinks = append(trackLinks, href)
							linkMap[href] = true
							if len(trackLinks) <= 5 || len(trackLinks)%10 == 0 {
								log.Printf("🔍 [EXTRACT BREAKPOINT 6.%d] Added track link: %s", i+1, href)
							}
						}
					}
				}
			} else {
				log.Printf("🔍 [EXTRACT BREAKPOINT 6] JavaScript evaluation successful!")

				// Convert JavaScript result to string slice
				if resultSlice, ok := jsResult.([]interface{}); ok {
					log.Printf("🔍 [EXTRACT BREAKPOINT 6] Found %d track links via JavaScript", len(resultSlice))
					linkMap := make(map[string]bool)

					for i, item := range resultSlice {
						if href, ok := item.(string); ok && href != "" {
							// Only add if unique
							if !linkMap[href] {
								trackLinks = append(trackLinks, href)
								linkMap[href] = true
								if len(trackLinks) <= 5 || len(trackLinks)%10 == 0 {
									log.Printf("🔍 [EXTRACT BREAKPOINT 6.%d] Added track link: %s", i+1, href)
								}
							}
						}
					}
				} else {
					log.Printf("🔍 [EXTRACT BREAKPOINT 6] Unexpected JavaScript result type: %T", jsResult)
					return fmt.Errorf("unexpected JavaScript result type")
				}
			}

			log.Printf("🔍 [EXTRACT BREAKPOINT 7] Extraction complete! Found %d unique track links", len(trackLinks))

			// Log summary
			if len(trackLinks) > 0 {
				log.Printf("🔍 [EXTRACT BREAKPOINT 8] First track: %s", trackLinks[0])
				if len(trackLinks) > 1 {
					log.Printf("🔍 [EXTRACT BREAKPOINT 8] Last track: %s", trackLinks[len(trackLinks)-1])
				}
			}

			return nil
		}),
	)

	return trackLinks, err
}
