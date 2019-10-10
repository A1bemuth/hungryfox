package deps

import (
	ossindex "github.com/A1bemuth/go-oss-index"
	osstypes "github.com/A1bemuth/go-oss-index/types"
	"github.com/AlexAkulov/hungryfox"
	"github.com/rs/zerolog"
)

type Credentials struct {
	User     string
	Password string
}

type VulnerabilitySearcher struct {
	VulnerabilitiesChannel chan<- *hungryfox.VulnerableDependency
	Log                    zerolog.Logger

	ossIndexClient ossindex.Client
}

func NewSearcher(vulnsChan chan<- *hungryfox.VulnerableDependency, log zerolog.Logger, ossCredentials Credentials) *VulnerabilitySearcher {
	return &VulnerabilitySearcher{
		VulnerabilitiesChannel: vulnsChan,
		Log:                    log,
		ossIndexClient: ossindex.Client{
			User:     ossCredentials.User,
			Password: ossCredentials.Password,
		},
	}
}

func (s *VulnerabilitySearcher) Search(deps []hungryfox.Dependency) error {
	purls, depsMap := mapPurls(deps)
	reports, err := s.ossIndexClient.Get(purls)
	if err != nil {
		s.Log.Warn().Err(err).Msg("requesting oss index component reports failed")
		return err
	}
	for _, report := range reports {
		vulns := getVulnerabilities(&report)
		found := len(vulns)
		if found == 0 {
			continue
		}
		dep, ok := depsMap[report.Coordinates]
		if !ok {
			s.Log.Warn().Str("coordinates", report.Coordinates).Msg("found an oss report but no matching dependency")
			continue
		}
		s.Log.Debug().Str("file", dep.FilePath).Int("count", found).Msg("vulnerabilities found")
		s.VulnerabilitiesChannel <- toVulnerableDep(dep, vulns)
	}

	return nil
}

func mapPurls(deps []hungryfox.Dependency) ([]string, map[string]*hungryfox.Dependency) {
	purls := make([]string, len(deps))
	depsMap := make(map[string]*hungryfox.Dependency)
	for i, dep := range deps {
		purl := dep.Purl.ToString()
		purls[i] = purl
		depsMap[purl] = &dep
	}
	return purls, depsMap
}

func toVulnerableDep(dep *hungryfox.Dependency, vulns []hungryfox.Vulnerability) *hungryfox.VulnerableDependency {
	return &hungryfox.VulnerableDependency{
		Vulnerabilities: vulns,
		DependencyName:  dep.Purl.Name,
		Version:         dep.Purl.Version,
		FilePath:        dep.Diff.FilePath,
		RepoPath:        dep.Diff.RepoPath,
		RepoURL:         dep.Diff.RepoURL,
		CommitHash:      dep.Diff.CommitHash,
		TimeStamp:       dep.Diff.TimeStamp,
		CommitAuthor:    dep.Diff.Author,
		CommitEmail:     dep.Diff.AuthorEmail,
	}
}

func getVulnerabilities(report *osstypes.ComponentReport) (vulns []hungryfox.Vulnerability) {
	for _, vuln := range report.Vulnerabilities {
		vulns = append(vulns, *toVulnerability(&vuln))
	}
	return vulns
}

func toVulnerability(vuln *osstypes.Vulnerability) *hungryfox.Vulnerability {
	return &hungryfox.Vulnerability{
		Source:        "Sonatype OSS Index",
		Id:            vuln.Id,
		Title:         vuln.Title,
		Description:   vuln.Description,
		CvssScore:     vuln.CvssScore,
		CvssVector:    vuln.CvssVector,
		Cwe:           vuln.Cwe,
		Cve:           vuln.Cve,
		Reference:     vuln.Reference,
		VersionRanges: vuln.VersionRanges,
	}
}