package imagelist

const imageRefsReport = `
# Invalid Image References Report
#
# This file contains image references that were found in chart values.yaml files
# but failed validation. These are typically placeholder values that need to be
# fixed in the source charts.
#
# Format:
# Image: <invalid-image-reference>
# Reason: <why-it-failed-validation>
# Catalog: <cluster-repo-catalog>
# Sources: <chart:version> [<chart:version> ...]
#
# Total invalid entries: {{ .InvalidCount }}

`

const imageRefItem = `

Image: {{ .Image }}
Reason: {{ .Reason }}
Catalog: {{ .CatalogRef }}
Sources: {{ .Sources }}
`
