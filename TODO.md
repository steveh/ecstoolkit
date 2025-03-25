* [ ] https://github.com/aws/session-manager-plugin/pull/16
* [ ] https://github.com/aws/session-manager-plugin/pull/38
* [ ] https://github.com/aws/session-manager-plugin/pull/54
* [ ] https://github.com/aws/session-manager-plugin/pull/80
* [ ] https://github.com/aws/session-manager-plugin/pull/96
* [ ] https://github.com/aws/session-manager-plugin/pull/97
* [ ] https://github.com/aws/session-manager-plugin/pull/105


Error Handling Issues (High Priority, Easy to Fix)
Unchecked error returns (errcheck): ~20 instances
Error wrapping issues (wrapcheck): ~30 instances
Dynamic error creation (err113): ~15 instances
Error string capitalization (stylecheck): ~3 instances
Global Variables (Medium Priority, Moderate to Fix)
Global variables (gochecknoglobals): ~20 instances
Init function usage (gochecknoinits): ~3 instances
Interface Issues (Medium Priority, Moderate to Fix)
Interface bloat (interfacebloat): ~2 instances
Interface method parameter naming (inamedparam): ~10 instances
Interface returns (ireturn): ~10 instances
Code Style and Best Practices (Low Priority, Easy to Fix)
Named returns (nonamedreturns): ~15 instances
Magic numbers (mnd): ~10 instances
Unnecessary conversions (unconvert): ~3 instances
Redundant return statements (gosimple): ~1 instance
Duplicate code (dupl): ~1 instance
Security Issues (High Priority, Moderate to Fix)
Weak random number generator usage (gosec): ~2 instances
Potential tainted input (gosec): ~1 instance
Integer overflow (gosec): ~3 instances
Slice index out of range (gosec): ~3 instances
Testing Issues (Low Priority, Easy to Fix)
Test package naming (testpackage): ~8 instances
Float comparison in tests (testifylint): ~2 instances
Test environment setup (usetesting): ~2 instances
Deprecated Package Usage (Medium Priority, Easy to Fix)
Deprecated package usage (staticcheck): ~3 instances
