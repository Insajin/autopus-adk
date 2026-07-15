package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func prepareSnapshotProofReporter(tempDir string) (reporterPath, proofPath, nonce string, err error) {
	nonceBytes := make([]byte, 32)
	if _, err = rand.Read(nonceBytes); err != nil {
		return "", "", "", fmt.Errorf("snapshot proof nonce 생성 실패: %w", err)
	}
	nonce = hex.EncodeToString(nonceBytes)
	reporterPath = filepath.Join(tempDir, "snapshot-proof-reporter.cjs")
	proofPath = filepath.Join(tempDir, "snapshot-proof.json")
	if err = os.WriteFile(reporterPath, []byte(snapshotProofReporterSource()), 0o600); err != nil {
		return "", "", "", fmt.Errorf("snapshot proof reporter 생성 실패: %w", err)
	}
	return reporterPath, proofPath, nonce, nil
}

func snapshotProofReporterSource() string {
	return `'use strict';
const fs = require('node:fs');
module.exports = class AutopusSnapshotProofReporter {
  constructor() {
    this.outputPath = process.env.AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_FILE;
    this.nonce = process.env.AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_NONCE;
    delete process.env.AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_FILE;
    delete process.env.AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_NONCE;
  }
  onBegin(config, suite) {
	const playwrightVersion = typeof config.version === 'string' && config.version ? config.version : 'unavailable';
	const allowedUpdates = new Set(['none', 'missing', 'changed', 'all']);
	const updateSnapshots = allowedUpdates.has(config.updateSnapshots) ? config.updateSnapshots : 'missing';
	const projectSuites = Array.isArray(suite.suites) ? suite.suites : [];
    const rows = projectSuites.map((projectSuite, index) => {
	  const hasPublicProject = typeof projectSuite.project === 'function';
	  const project = hasPublicProject ? projectSuite.project() : null;
	  const publicTitle = typeof projectSuite.title === 'string' && projectSuite.title ? projectSuite.title : 'project';
	  const name = project && typeof project.name === 'string' && project.name
	    ? project.name : publicTitle + ' [unsupported:' + index + ']';
      const ignoreSnapshots = project && typeof project.ignoreSnapshots === 'boolean' ? project.ignoreSnapshots : null;
      const source = ignoreSnapshots === null ? 'unsupported' : 'public';
      const state = source !== 'public' ? 'unproven'
        : ignoreSnapshots ? 'disabled'
		: updateSnapshots === 'none' ? 'enabled' : 'unproven';
      return {
		name,
        ignore_snapshots: ignoreSnapshots,
        state,
        source,
		dependencies: project && Array.isArray(project.dependencies) ? project.dependencies.slice() : [],
		teardown: project && typeof project.teardown === 'string' ? project.teardown : '',
      };
    });
    const supportNames = new Set();
    rows.forEach(project => {
      project.dependencies.forEach(name => supportNames.add(name));
      if (project.teardown) supportNames.add(project.teardown);
    });
    const projects = rows.map(project => ({ ...project, support_only: supportNames.has(project.name) }));
    fs.writeFileSync(this.outputPath, JSON.stringify({
      version: 2,
      nonce: this.nonce,
	  playwright_version: playwrightVersion,
	  update_snapshots: updateSnapshots,
      projects,
    }), { encoding: 'utf8', mode: 0o600, flag: 'wx' });
  }
};
`
}
