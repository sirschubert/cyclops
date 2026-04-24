package hosts

import (
	"strings"

	"github.com/sirschubert/cyclops/pkg/models"
)

// Fingerprint performs basic technology fingerprinting on a host
func Fingerprint(host *models.Host) []string {
	var technologies []string

	// Check server header
	server := strings.ToLower(host.Server)
	if server != "" {
		technologies = append(technologies, identifyTechByServer(server)...)
	}

	// Check other headers
	for header, value := range host.Headers {
		header = strings.ToLower(header)
		value = strings.ToLower(value)

		// X-Powered-By header
		if header == "x-powered-by" {
			technologies = append(technologies, identifyTechByHeader(value)...)
		}

		// X-Generator header
		if header == "x-generator" {
			technologies = append(technologies, identifyTechByHeader(value)...)
		}
	}

	// Check response body for common signatures
	body := strings.ToLower(host.BodyPreview)
	if body != "" {
		technologies = append(technologies, identifyTechByBody(body)...)
	}

	// Deduplicate technologies
	techMap := make(map[string]bool)
	for _, tech := range technologies {
		techMap[tech] = true
	}

	var uniqueTech []string
	for tech := range techMap {
		uniqueTech = append(uniqueTech, tech)
	}

	return uniqueTech
}

// identifyTechByServer identifies technology by server header
func identifyTechByServer(server string) []string {
	var technologies []string

	// Web servers
	if strings.Contains(server, "apache") {
		technologies = append(technologies, "Apache")
		if strings.Contains(server, "tomcat") {
			technologies = append(technologies, "Tomcat")
		}
	}

	if strings.Contains(server, "nginx") {
		technologies = append(technologies, "Nginx")
	}

	if strings.Contains(server, "iis") || strings.Contains(server, "microsoft") {
		technologies = append(technologies, "IIS")
	}

	if strings.Contains(server, "lighttpd") {
		technologies = append(technologies, "Lighttpd")
	}

	// Application frameworks
	if strings.Contains(server, "express") {
		technologies = append(technologies, "Express.js")
	}

	if strings.Contains(server, "gunicorn") {
		technologies = append(technologies, "Gunicorn")
	}

	if strings.Contains(server, "jetty") {
		technologies = append(technologies, "Jetty")
	}

	return technologies
}

// identifyTechByHeader identifies technology by specific headers
func identifyTechByHeader(header string) []string {
	var technologies []string

	if strings.Contains(header, "php") {
		technologies = append(technologies, "PHP")
	}

	if strings.Contains(header, "asp.net") {
		technologies = append(technologies, "ASP.NET")
	}

	if strings.Contains(header, "django") {
		technologies = append(technologies, "Django")
	}

	if strings.Contains(header, "ror") || strings.Contains(header, "ruby") {
		technologies = append(technologies, "Ruby on Rails")
	}

	if strings.Contains(header, "laravel") {
		technologies = append(technologies, "Laravel")
	}

	if strings.Contains(header, "spring") {
		technologies = append(technologies, "Spring Boot")
	}

	if strings.Contains(header, "next.js") || strings.Contains(header, "nextjs") {
		technologies = append(technologies, "Next.js")
	}

	if strings.Contains(header, "nuxt") {
		technologies = append(technologies, "Nuxt.js")
	}

	return technologies
}

// identifyTechByBody identifies technology by response body content
func identifyTechByBody(body string) []string {
	var technologies []string

	// JavaScript frameworks
	if strings.Contains(body, "react") && (strings.Contains(body, "react-dom") || strings.Contains(body, "reactroot")) {
		technologies = append(technologies, "React")
	}

	if strings.Contains(body, "angular") && (strings.Contains(body, "ng-version") || strings.Contains(body, "ng-app")) {
		technologies = append(technologies, "Angular")
	}

	if strings.Contains(body, "vue.js") || strings.Contains(body, "vue.min.js") {
		technologies = append(technologies, "Vue.js")
	}

	// CMS platforms
	if strings.Contains(body, "wp-content") || strings.Contains(body, "wordpress") {
		technologies = append(technologies, "WordPress")
	}

	if strings.Contains(body, "drupal") || strings.Contains(body, "sites/all/") {
		technologies = append(technologies, "Drupal")
	}

	if strings.Contains(body, "joomla") {
		technologies = append(technologies, "Joomla")
	}

	// Frontend frameworks
	if strings.Contains(body, "bootstrap") {
		technologies = append(technologies, "Bootstrap")
	}

	if strings.Contains(body, "jquery") {
		technologies = append(technologies, "jQuery")
	}

	// Cloud services
	if strings.Contains(body, "cloudflare") {
		technologies = append(technologies, "Cloudflare")
	}

	if strings.Contains(body, "aws") || strings.Contains(body, "amazon") {
		technologies = append(technologies, "AWS")
	}

	// Analytics
	if strings.Contains(body, "googletagmanager.com") || strings.Contains(body, "google-analytics.com") {
		technologies = append(technologies, "Google Analytics")
	}

	return technologies
}

// FingerprintHosts performs fingerprinting on multiple hosts
func FingerprintHosts(hosts []models.Host) []models.Host {
	for i := range hosts {
		tech := Fingerprint(&hosts[i])
		hosts[i].Tech = tech
	}
	return hosts
}