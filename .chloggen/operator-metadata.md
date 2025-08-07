# Use this changelog template to create an entry for release notes.

# One of 'breaking', 'deprecation', 'new_component', 'enhancement', 'bug_fix'
change_type: enhancement

# The name of the component, or a single word describing the area of concern, (e.g. filelogreceiver)
component: config_examples

# A brief description of the change.  Surround your text with quotes ("") if it needs to start with a backtick (`).
note: Add Dynatrace Operator metadata object processing to k8s enrichment config example

# Mandatory: One or more tracking issues related to the change. You can use the PR number here if no issue exists.
issues: [615]

# (Optional) One or more lines of additional information to render under the primary note.
# These lines will be padded with 2 spaces and then inserted directly into the document.
# Use pipe (|) for multiline entries.
subtext: This extends the config example for the k8sattributesprocessor and related transformprocessor statements
| to add pod metadata added by the Dynatrace Operator to the resource attributes in exported telemetry. See
| https://docs.dynatrace.com/docs/shortlink/k8s-metadata-telemetry-enrichment on how to configure this operator feature.
