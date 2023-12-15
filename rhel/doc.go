// Package rhel is meant to be a TEMPORARY divergence from upstream ClairCore.
// This package is introduced as a simple workaround for obtaining Red Hat's CVSS scores
// for vulnerabilities, as it tends to be preferred to use Red Hat's CVSS scores
// for Red Hat products instead of NVD's.
//
// This package is expected to only exist temporarily until ClairCore switches over from
// the current OVAL v2 feeds to the upcoming CSAF/VEX feeds https://access.redhat.com/security/data/csaf/.
// This change is expected to take place early 2024, so the StackRox team is willing to
// live with this temporary divergence, as we know the CSAF/VEX migration will happen.
//
// The contents of this package are almost entirely copied from
// https://github.com/quay/claircore/tree/8fd9a12427a036b9a8456cf60a555bddc2fcdf0c/rhel.
//
// The significant change made here is the [Severity] field's contents are replaced with
// the CVSS base score/vector.
package rhel
