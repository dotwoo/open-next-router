package dslconfig

import "strings"

func parseModelsPhase(s *scanner, cfg *ModelsQueryConfig) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after models")
	}

	var hdr PhaseHeaders
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in models phase")
		case tokRBrace:
			if len(hdr.Request) > 0 {
				cfg.Headers = append(cfg.Headers, hdr.Request...)
			}
			return nil
		case tokIdent:
			switch tok.text {
			case "models_mode":
				mode, err := parseModeArgStmt(s, "models_mode")
				if err != nil {
					return err
				}
				cfg.Mode = strings.TrimSpace(mode)
			case "method", "path", "id_regex", "id_allow_regex":
				v, err := parseBalanceFieldStmt(s, tok.text)
				if err != nil {
					return err
				}
				switch tok.text {
				case "method":
					cfg.Method = v
				case "path":
					cfg.Path = v
				case "id_regex":
					cfg.IDRegex = v
				case "id_allow_regex":
					cfg.IDAllowRegex = v
				}
			case "id_path":
				v, err := parseBalanceFieldStmt(s, tok.text)
				if err != nil {
					return err
				}
				v = strings.TrimSpace(v)
				if v != "" {
					cfg.IDPaths = append(cfg.IDPaths, v)
				}
			case "set_header":
				if err := parseSetHeaderStmt(s, &hdr); err != nil {
					return err
				}
			case "del_header":
				if err := parseDelHeaderStmt(s, &hdr); err != nil {
					return err
				}
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return err
				}
			}
		default:
			// ignore
		}
	}
}
