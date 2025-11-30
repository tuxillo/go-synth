package service

import (
	"fmt"

	"dsynth/builddb"
)

// GetStatus retrieves build status information from the database.
//
// If opts.PortList is empty, returns overall database statistics.
// If opts.PortList contains port directories, returns status for each specified port.
//
// This method handles all the business logic but does not interact with the user.
// The caller is responsible for:
//   - Formatting and displaying the status information
//   - Handling cases where ports have never been built
//
// Returns StatusResult containing database stats and/or port-specific status.
func (s *Service) GetStatus(opts StatusOptions) (*StatusResult, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	result := &StatusResult{
		Ports: make([]PortStatus, 0),
	}

	// If no specific ports requested, return overall database statistics
	if len(opts.PortList) == 0 {
		stats, err := s.db.Stats()
		if err != nil {
			return nil, fmt.Errorf("failed to get database stats: %w", err)
		}
		result.Stats = stats
		result.DatabaseSize = stats.DatabaseSize
		return result, nil
	}

	// Return status for specific ports
	for _, portDir := range opts.PortList {
		status := PortStatus{
			PortDir: portDir,
		}

		// Get latest build record
		rec, err := s.db.LatestFor(portDir, "")
		if err != nil || rec == nil {
			// Port has never been built
			status.LastBuild = nil
		} else {
			status.LastBuild = rec
			status.Version = rec.Version
		}

		// Get CRC if available
		if crc, exists, err := s.db.GetCRC(portDir); err == nil && exists {
			status.CRC = crc
		}

		// TODO: Determine if port needs building (would require parsing the port)
		// For now, we leave NeedsBuild as false
		status.NeedsBuild = false

		result.Ports = append(result.Ports, status)
	}

	return result, nil
}

// GetDatabaseStats returns overall database statistics without port-specific information.
func (s *Service) GetDatabaseStats() (*builddb.DBStats, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	stats, err := s.db.Stats()
	if err != nil {
		return nil, fmt.Errorf("failed to get database stats: %w", err)
	}

	return stats, nil
}

// GetPortStatus returns status for a single port.
func (s *Service) GetPortStatus(portDir string) (*PortStatus, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	status := &PortStatus{
		PortDir: portDir,
	}

	// Get latest build record
	rec, err := s.db.LatestFor(portDir, "")
	if err != nil || rec == nil {
		// Port has never been built
		status.LastBuild = nil
	} else {
		status.LastBuild = rec
		status.Version = rec.Version
	}

	// Get CRC if available
	if crc, exists, err := s.db.GetCRC(portDir); err == nil && exists {
		status.CRC = crc
	}

	return status, nil
}
