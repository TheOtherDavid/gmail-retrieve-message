package gmail-retrieve-message

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	sender := os.Getenv("TARGET_SENDER")

	message := RetrieveUnreadMessageFromSender(sender)
	fmt.Println(message)

	artists := extractArtists(message)
	if len(artists) > 0 {
		fmt.Println(artists)
	} else {
		fmt.Println("No artists found!")
	}
}

func RetrieveUnreadMessageFromSender(sender string) (message string) {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Gmail client: %v", err)
	}

	user := "me"
	r, err := srv.Users.Labels.List(user).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve labels: %v", err)
	}
	if len(r.Labels) == 0 {
		fmt.Println("No labels found.")
		return
	}
	fmt.Println("Labels:")
	for _, l := range r.Labels {
		fmt.Printf("- %s\n", l.Name)
	}

	// Get the list of messages in the inbox.
	result, err := srv.Users.Messages.List("me").LabelIds("INBOX").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
	}

	// Check for the first unread email from the specified sender.
	if len(result.Messages) > 0 {
		for _, message := range result.Messages {
			msg, err := srv.Users.Messages.Get("me", message.Id).Do()
			if err != nil {
				log.Fatalf("Unable to retrieve message: %v", err)
			}
			if !containsLabel(msg.LabelIds, "INBOX") || containsLabel(msg.LabelIds, "UNREAD") {
				isFromSender := false
				for _, header := range msg.Payload.Headers {
					if header.Name == "From" {
						if header.Value == sender {
							isFromSender = true
							break
						}
					}
				}
				if !isFromSender {
					continue // skip this message, move to the next one
				}
				encodedBody := msg.Payload.Body.Data
				body, err := base64.URLEncoding.DecodeString(encodedBody)
				if err != nil {
					log.Fatalf("Unable to decode message body: %v", err)
				}
				bodyString := string(body[:])
				fmt.Println("Decoded body: " + bodyString)
				return bodyString
			}
		}
	}
	return ""

}

// containsLabel returns true if the given list of labels contains the specified label.
func containsLabel(labels []string, label string) bool {
	for _, l := range labels {
		if l == label {
			return true
		}
	}
	return false
}

func extractArtists(s string) []string {
	// Split the string into lines
	lines := strings.Split(s, "\n")

	// Initialize a slice to store the artist names
	artists := make([]string, 0)

	// Iterate over the lines
	for _, line := range lines {
		// Check if the line starts with an uppercase letter
		if len(line) > 0 && line[0] >= 'A' && line[0] <= 'Z' {
			// If it does, split the line into sections, potential artist names, separated by commas
			potentialArtists := strings.Split(line, ",")

			// Iterate over the potential artists
			for _, potentialArtist := range potentialArtists {
				potentialArtist = strings.TrimSpace(potentialArtist)
				// Check if the potential artist ends with ")"
				if strings.HasSuffix(potentialArtist, ")") {
					// If it does, remove the parenthesis and add the artist to the slice
					trimmedArtist := trimCountry(potentialArtist)
					artists = append(artists, trimmedArtist)
				}
			}
		}
	}
	return artists
}

func trimCountry(s string) string {
	// Find the first index of " ("
	i := strings.Index(s, " (")
	// Return a substring that starts at the beginning of the string and ends at the index of " ("
	return s[:i]
}
