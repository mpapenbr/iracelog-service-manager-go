//nolint:lll,funlen // readablity
package traefik

import "testing"

func TestGetCertData(t *testing.T) {
	type args struct {
		jsonData string
		domain   string
	}
	tests := []struct {
		name    string
		args    args
		cert    string
		key     string
		wantErr bool
	}{
		{
			name: "Success",
			args: args{
				jsonData: `{"dummy":{"Certificates":[{"domain":{"main":"example.com"}, "certificate": "cert1", "key": "key1"}]}}`,
				domain:   "example.com",
			},
			cert:    "cert1",
			key:     "key1",
			wantErr: false,
		},
		{
			name: "Wildcard domain",
			args: args{
				jsonData: `{"myresolver":{"Certificates":[{"domain":{"main":"*.example.com"}, "certificate": "cert1", "key": "key1"}]}}`,
				domain:   "*.example.com",
			},
			cert:    "cert1",
			key:     "key1",
			wantErr: false,
		},
		{
			name: "Domain not found",
			args: args{
				jsonData: `{"dummy":{"Certificates":[{"domain":{"main":"example.com"}, "certificate": "cert1", "key": "key1"}]}}`,
				domain:   "notfound.com",
			},
			cert:    "",
			key:     "",
			wantErr: true,
		},
		{
			name: "Empty json",
			args: args{
				jsonData: `{}`,
				domain:   "notfound.com",
			},
			cert:    "",
			key:     "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := getCertData(tt.args.jsonData, tt.args.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCertData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.cert {
				t.Errorf("GetCertData() got = %v, want %v", got, tt.cert)
			}
			if got1 != tt.key {
				t.Errorf("GetCertData() got1 = %v, want %v", got1, tt.key)
			}
		})
	}
}
