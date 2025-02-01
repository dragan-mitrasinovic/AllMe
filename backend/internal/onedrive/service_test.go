package onedrive

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeFolderLink(t *testing.T) {
	tests := []struct {
		name       string
		folderLink string
		expected   string
	}{
		{
			name:       "Basic URL",
			folderLink: "https://example.com/folder",
			expected:   "u!aHR0cHM6Ly9leGFtcGxlLmNvbS9mb2xkZXI",
		},
		{
			name:       "Example with 1drv.ms link",
			folderLink: "https://1drv.ms/f/c/cbe20023e15f677d/EpdOZSAgW4FLgPqbLn510DwBM8V_8y-hbQJ3-MvyC6GwRw",
			expected:   "u!aHR0cHM6Ly8xZHJ2Lm1zL2YvYy9jYmUyMDAyM2UxNWY2NzdkL0VwZE9aU0FnVzRGTGdQcWJMbjUxMER3Qk04Vl84eS1oYlFKMy1NdnlDNkd3Unc",
		},
		{
			name:       "Example with onderive.live.com link",
			folderLink: "https://onedrive.live.com/?redeem=aHR0cHM6Ly8xZHJ2Lm1zL2YvYy9jYmUyMDAyM2UxNWY2NzdkL0VwZE9aU0FnVzRGTGdQcWJMbjUxMER3Qk04Vl84eS1oYlFKMy1NdnlDNkd3Unc&id=CBE20023E15F677D%21s20654e975b204b8180fa9b2e7e75d03c&cid=CBE20023E15F677D",
			expected:   "u!aHR0cHM6Ly9vbmVkcml2ZS5saXZlLmNvbS8_cmVkZWVtPWFIUjBjSE02THk4eFpISjJMbTF6TDJZdll5OWpZbVV5TURBeU0yVXhOV1kyTnpka0wwVndaRTlhVTBGblZ6UkdUR2RRY1dKTWJqVXhNRVIzUWswNFZsODRlUzFvWWxGS015MU5kbmxETmtkM1VuYyZpZD1DQkUyMDAyM0UxNUY2NzdEJTIxczIwNjU0ZTk3NWIyMDRiODE4MGZhOWIyZTdlNzVkMDNjJmNpZD1DQkUyMDAyM0UxNUY2NzdE",
		},
		{
			name:       "Empty string",
			folderLink: "",
			expected:   "u!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encodedLink := encodeFolderLink(tt.folderLink)
			assert.Equal(t, tt.expected, encodedLink)
		})
	}
}

func TestGetImagesFromSharedFolder(t *testing.T) {
	tests := []struct {
		name           string
		mockStatusCode int
		mockResponse   interface{}
		expectedError  error
		expectedItems  []DriveImage
	}{
		{
			name:           "Happy Path",
			mockStatusCode: http.StatusOK,
			mockResponse: FolderContentsResponse{
				Value: []DriveItem{
					{Name: "image1.jpg", File: File{MimeType: "image/jpeg"}, Source: "http://example.com/image1.jpg"},
					{Name: "image2.png", File: File{MimeType: "image/png"}, Source: "http://example.com/image2.png"},
					{Name: "file3.txt", File: File{MimeType: "text/plain"}, Source: "http://example.com/file3.txt"},
				},
			},
			expectedError: nil,
			expectedItems: []DriveImage{
				{Name: "image1.jpg", Source: "http://example.com/image1.jpg"},
				{Name: "image2.png", Source: "http://example.com/image2.png"},
			},
		},
		{
			name:           "Real Example",
			mockStatusCode: http.StatusOK,
			mockResponse:   loadMockResponseFromFile(t, "../../testdata/onedrive_response.json"),
			expectedError:  nil,
			expectedItems: []DriveImage{
				{Name: "test_picture_1.jpg", Source: "https://my.microsoftpersonalcontent.com/personal/cbe20023e15f677d/_layouts/15/download.aspx?UniqueId=e466bb82-2600-45d2-bb87-193ad32e2879&Translate=false&tempauth=v1e.eyJzaXRlaWQiOiI3YWU5YjFiYi02OTdlLTQ0M2YtYjMyMi01ZmY3MTVjODQxOTciLCJhcHBfZGlzcGxheW5hbWUiOiJHcmFwaCIsImFwcGlkIjoiNjBhZGYxMWMtZTU1NC00Nzc0LWI5ZmQtOGZhNTRmOGFmYTdkIiwiYXVkIjoiMDAwMDAwMDMtMDAwMC0wZmYxLWNlMDAtMDAwMDAwMDAwMDAwL215Lm1pY3Jvc29mdHBlcnNvbmFsY29udGVudC5jb21AOTE4ODA0MGQtNmM2Ny00YzViLWIxMTItMzZhMzA0YjY2ZGFkIiwiZXhwIjoiMTczODQ1Mzk1NSJ9.v1eLjJs7wuqheFQ3QMlggCw9lyObxAIwpAR_4uscPD2dti3TV3pz97WXo2-Plgvte1wnzNuTfhP7HjaSjG6HbAkyJAyNH9ue7C5V17tEqvPcPlinPnz3fNCHnBIhZUwJVgqq45t99rzXvSP6y-cvpJbNSRVpBwkUrnTjPlwoq7Qrm9ALuf__b7b4xhHY_pcI_PUWgjDzUcI-llqdLLeo07rP1HTHZ51HcsbjcKWsIKD0Odi9VsNL1dIz3ICDfE4C5TV2mYs6lzqY2nMz8GgzgQvI20GNL8J82XDpYu-8VSRPWWa3_mpeJcLFmJUvopXT9W-o33aIlkLvd9GNEF1w1rijxg2TAY9D0YI_ctHgU7Fn8zbZF_BianCO6uuyt5d7JxmRd9cwRxmKr7sLwkF7mHkhlRVU80-HDH-okc7UWdJEKLagPCO26PyZTfoSP_sTz1qYlRme0jJn53F08HkZLQ._vzrzFheFcODuFWDqa7uraUHuIo8H8Jgbabu6jSeC-U&ApiVersion=2.0"},
			},
		},
		{
			name:           "Bad Auth",
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   nil,
			expectedError:  errors.New("received non-200 response code"),
			expectedItems:  nil,
		},
		{
			name:           "Failed Request",
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   nil,
			expectedError:  errors.New("received non-200 response code"),
			expectedItems:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			client := server.Client()
			service := NewService(client, server.URL)

			items, err := service.GetImagesFromSharedFolder("encodedFolderLink", "authToken")

			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedItems, items)
		})
	}
}

func loadMockResponseFromFile(t *testing.T, s string) interface{} {
	t.Helper()

	file, err := os.Open(s)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer file.Close()

	var response interface{}
	if err := json.NewDecoder(file).Decode(&response); err != nil {
		t.Fatalf("failed to decode file: %v", err)
	}

	return response
}
