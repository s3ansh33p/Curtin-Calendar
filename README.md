# Curtin Calendar

Merges ical files from different clubs into calendars grouped by category.

## Setup

1. Install go
2. Create an R2 Bucket on Cloudflare and link to a domain
3. Fill a .env file with the following variables from Cloudflare:
    - ACCESS_KEY
    - SECRET_KEY
    - ACCOUNT_ID
    - BUCKET_NAME
4. Build with `go build -o curtin-calendar`
5. Run with `./curtin-calendar` or create a cron job to run it periodically `crontab -e`:

  ```bash
  0 * * * * cd /path/to/repo && ./curtin-calendar
  ```

6. Success! Calendar files should be available at `https://yourdomain.com/calendarname.ics`
