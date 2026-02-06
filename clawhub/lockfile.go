package clawhub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Lockfile represents the .clawhub/lock.json file
type Lockfile struct {
	Version string            `json:"version"`
	Skills  map[string]Skill  `json:"skills"`
}

// Skill represents an installed skill in the lockfile
type Skill struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
	Hash        string    `json:"hash,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
}

// NewLockfile creates a new lockfile
func NewLockfile() *Lockfile {
	return &Lockfile{
		Version: "1.0.0",
		Skills:  make(map[string]Skill),
	}
}

// LoadLockfile loads the lockfile from the workdir
func LoadLockfile(workdir string) (*Lockfile, error) {
	lockfilePath := filepath.Join(workdir, LockfileDir, LockfileName)

	// If lockfile doesn't exist, return a new one
	if _, err := os.Stat(lockfilePath); os.IsNotExist(err) {
		return NewLockfile(), nil
	}

	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read lockfile: %w", err)
	}

	var lf Lockfile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("failed to parse lockfile: %w", err)
	}

	// Initialize skills map if nil
	if lf.Skills == nil {
		lf.Skills = make(map[string]Skill)
	}

	return &lf, nil
}

// Save saves the lockfile to the workdir
func (lf *Lockfile) Save(workdir string) error {
	lockfileDir := filepath.Join(workdir, LockfileDir)

	// Ensure lockfile directory exists
	if err := os.MkdirAll(lockfileDir, 0755); err != nil {
		return fmt.Errorf("failed to create lockfile directory: %w", err)
	}

	lockfilePath := filepath.Join(lockfileDir, LockfileName)

	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lockfile: %w", err)
	}

	if err := os.WriteFile(lockfilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write lockfile: %w", err)
	}

	return nil
}

// AddSkill adds a skill to the lockfile
func (lf *Lockfile) AddSkill(slug, name, version, hash string, tags []string) {
	lf.Skills[slug] = Skill{
		Name:        name,
		Version:     version,
		InstalledAt: time.Now(),
		Hash:        hash,
		Tags:        tags,
	}
}

// RemoveSkill removes a skill from the lockfile
func (lf *Lockfile) RemoveSkill(slug string) {
	delete(lf.Skills, slug)
}

// GetSkill returns a skill from the lockfile
func (lf *Lockfile) GetSkill(slug string) (Skill, bool) {
	skill, ok := lf.Skills[slug]
	return skill, ok
}

// HasSkill returns true if the skill is in the lockfile
func (lf *Lockfile) HasSkill(slug string) bool {
	_, ok := lf.Skills[slug]
	return ok
}

// ListSkills returns all skills in the lockfile
func (lf *Lockfile) ListSkills() map[string]Skill {
	return lf.Skills
}

// SkillCount returns the number of skills in the lockfile
func (lf *Lockfile) SkillCount() int {
	return len(lf.Skills)
}

// GetSkillVersion returns the version of an installed skill
func (lf *Lockfile) GetSkillVersion(slug string) (string, bool) {
	skill, ok := lf.Skills[slug]
	if !ok {
		return "", false
	}
	return skill.Version, true
}

// GetSkillHash returns the hash of an installed skill
func (lf *Lockfile) GetSkillHash(slug string) (string, bool) {
	skill, ok := lf.Skills[slug]
	if !ok {
		return "", false
	}
	return skill.Hash, true
}

// UpdateSkillVersion updates the version of an installed skill
func (lf *Lockfile) UpdateSkillVersion(slug, version, hash string, tags []string) {
	if skill, ok := lf.Skills[slug]; ok {
		skill.Version = version
		skill.Hash = hash
		if tags != nil {
			skill.Tags = tags
		}
		lf.Skills[slug] = skill
	}
}
