## ADDED Requirements
### Requirement: Accurate dashboard presentation
The dashboard SHALL display only authorized, verified data and distinguish unavailable from zero.
#### Scenario: Partial API failure
- WHEN a widget request fails
- THEN that widget shows unavailable/retry and successful widgets remain usable.
### Requirement: Responsive navigation
The dashboard SHALL expose permitted navigation at desktop and mobile sizes.
#### Scenario: Supervisor opens mobile dashboard
- WHEN role is resolved
- THEN only permitted links are shown and accessible by keyboard.
### Requirement: Explicit period scope
The dashboard SHALL label today metrics separately from trend range.
#### Scenario: Trend range changes
- WHEN a user selects 30 days
- THEN the trend updates and today metrics remain clearly labeled.
### Requirement: Accessible design
The dashboard SHALL use approved tokens and non-color status labels.
#### Scenario: Dark mode
- WHEN dark mode is enabled
- THEN text, charts and focus indicators remain legible.