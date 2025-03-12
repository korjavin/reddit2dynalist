# Reddit2Dynalist

Automatically sync your saved Reddit posts to Dynalist.

This application uses the [graw](https://github.com/turnage/graw) library to interact with the Reddit API.

## Configuration

The application requires the following environment variables:

```bash
# Reddit API credentials
REDDIT_CLIENT_ID=your_client_id
REDDIT_CLIENT_SECRET=your_client_secret
REDDIT_USERNAME=your_username
REDDIT_PASSWORD=your_password

# Dynalist API
DYNALIST_API_KEY=your_api_key
```

### Getting Reddit API Credentials

1. Go to https://www.reddit.com/prefs/apps
2. Click "create another app..." button at the bottom
3. Fill in the following:
   - name: reddit2dynalist (or any name you prefer)
   - select "script"
   - description: optional
   - about url: optional
   - redirect uri: http://localhost:8080 (any valid URL will work)
4. Click "create app"
5. In the created app, you will find:
   - client ID: the string under "personal use script"
   - client secret: the string labeled "secret"
   - Example:
     ```
     reddit2dynalist
     personal use script
     VGp7aHd8gQ_Fxw  <-- This is your client ID
     secret: kji9i9qqPOa_GfHk882991jj  <-- This is your client secret
     ```

### Getting Dynalist API Key

1. Go to https://dynalist.io/developer
2. Generate a new API token

## Running the Application

### Using Docker

```bash
docker build -t reddit2dynalist .
docker run -e REDDIT_CLIENT_ID=xxx \
           -e REDDIT_CLIENT_SECRET=xxx \
           -e REDDIT_USERNAME=xxx \
           -e REDDIT_PASSWORD=xxx \
           -e DYNALIST_API_KEY=xxx \
           reddit2dynalist
```

### Using GitHub Container Registry

You can also pull the pre-built image from GitHub Container Registry:

```bash
docker pull ghcr.io/OWNER/reddit2dynalist:latest
docker run -e REDDIT_CLIENT_ID=xxx \
           -e REDDIT_CLIENT_SECRET=xxx \
           -e REDDIT_USERNAME=xxx \
           -e REDDIT_PASSWORD=xxx \
           -e DYNALIST_API_KEY=xxx \
           ghcr.io/OWNER/reddit2dynalist:latest
```

Replace `OWNER` with your GitHub username.

### Without Docker

```bash
go build
./reddit2dynalist
```

The application will check for new saved Reddit posts every 5 minutes and add them to your Dynalist document named "Reddit".
