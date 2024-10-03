# Github Contributions Chart Data
This is a simple program that scrapes the Github contributions chart data from the user's profile page and returns it in JSON format.

You can welcome to use my hosted version: `https://gh-contributions-chart-data.fly.dev/contributions/<github-username>`

Data is cached for 1 hour.

Example: [https://gh-contributions-chart-data.fly.dev/contributions/prosavage](https://gh-contributions-chart-data.fly.dev/contributions/prosavage)

The format is as follows:
```json
{
    "contributions": [
        "2017": [
            {
                "date": "2017-01-01",
                "level": 0,
                "count": 0
            },
            ...
        ],
        ...
    ],
    "totals": {
        "2017": 3,
        "2018": 1341,
        ...
    }
}
```