# Chessmata Deployment Guide

## ✅ Deployment Complete!

Your Chessmata application has been successfully deployed to Fly.io under the **Metavert LLC** organization.

- **App URL**: https://chessmata.fly.dev/
- **Fly.io Dashboard**: https://fly.io/apps/chessmata/monitoring
- **GitHub Repository**: https://github.com/jonradoff/chessmata (private)

---

## 🌐 DNS Configuration for Namecheap

To point your custom domain **chessmata.com** to your Fly.io app, add these DNS records in Namecheap:

### Required DNS Records

| Type | Host | Value | TTL |
|------|------|-------|-----|
| **A** | @ | `37.16.9.248` | Automatic |
| **AAAA** | @ | `2a09:8280:1::d1:b51e:0` | Automatic |
| **A** | www | `37.16.9.248` | Automatic |
| **AAAA** | www | `2a09:8280:1::d1:b51e:0` | Automatic |

### Optional (For Faster Certificate Provisioning)

| Type | Host | Value | TTL |
|------|------|-------|-----|
| **CNAME** | _acme-challenge | `chessmata.com.xzyl2zr.flydns.net` | Automatic |
| **CNAME** | _acme-challenge.www | `www.chessmata.com.xzyl2zr.flydns.net` | Automatic |

### Steps to Add DNS Records in Namecheap:

1. Log into your Namecheap account
2. Go to **Domain List** and click **Manage** next to `chessmata.com`
3. Select the **Advanced DNS** tab
4. Click **Add New Record** for each entry above
5. For the `@` host, just use `@` (it represents the root domain)
6. For the `www` host, just enter `www`
7. Click the checkmark to save each record

### DNS Propagation

- DNS changes can take **5-60 minutes** to propagate
- SSL certificates will be automatically provisioned by Let's Encrypt once DNS is configured
- Check certificate status: `flyctl certs check chessmata.com --app chessmata`

---

## 🔒 HTTPS/SSL Configuration

**HTTPS is automatically configured!**

- Fly.io uses **Let's Encrypt** for free SSL certificates
- Certificates auto-renew every 90 days
- Force HTTPS is enabled in `fly.toml` (all HTTP traffic redirects to HTTPS)
- Once DNS is configured, certificates will be issued within minutes

---

## 🔐 Required Secrets (Action Needed)

The following secrets need to be set for the app to function properly:

### 1. MongoDB Connection String

```bash
flyctl secrets set MONGODB_URI="mongodb+srv://username:password@cluster.mongodb.net/chessmata" --app chessmata
```

**Get your MongoDB URI from:**
- MongoDB Atlas: https://cloud.mongodb.com/
- Go to your cluster → Connect → Connect your application
- Copy the connection string and replace `<username>` and `<password>`

### 2. Google OAuth Credentials (Optional)

If you want Google login to work:

```bash
flyctl secrets set \
  GOOGLE_CLIENT_ID="your-google-client-id" \
  GOOGLE_CLIENT_SECRET="your-google-client-secret" \
  --app chessmata
```

**Get Google OAuth credentials:**
1. Go to https://console.cloud.google.com/
2. Create a project (or use existing)
3. Enable Google+ API
4. Go to Credentials → Create Credentials → OAuth 2.0 Client ID
5. Application type: Web application
6. Authorized redirect URIs: `https://chessmata.com/api/auth/google/callback`
7. Copy Client ID and Client Secret

### Already Set ✓

These secrets were automatically generated during deployment:

- ✅ `JWT_ACCESS_SECRET` - Secure random token for access tokens
- ✅ `JWT_REFRESH_SECRET` - Secure random token for refresh tokens

---

## 📊 Monitoring & Management

### View Logs

```bash
flyctl logs --app chessmata
```

### Check App Status

```bash
flyctl status --app chessmata
```

### Scale App

```bash
# Adjust memory/CPU
flyctl scale vm shared-cpu-1x --app chessmata --memory 1024

# Set number of machines
flyctl scale count 2 --app chessmata
```

### Restart App

```bash
flyctl apps restart chessmata
```

### SSH into Machine

```bash
flyctl ssh console --app chessmata
```

---

## 🚀 Deploying Updates

After making changes to your code:

```bash
# 1. Commit changes
git add .
git commit -m "Your update message"
git push

# 2. Deploy to Fly.io
flyctl deploy --app chessmata
```

**Note:** Deployments use `--remote-only` flag to build in Fly's infrastructure (faster and doesn't use local resources).

---

## 📝 Configuration Files

- **`fly.toml`** - Fly.io configuration
- **`Dockerfile`** - Multi-stage build (frontend + backend)
- **`backend/configs/config.prod.example.json`** - Production config template
- **`.gitignore`** - Excludes secrets and builds from git

---

## 🏗️ Architecture

**Multi-stage Docker Build:**

1. **Frontend Builder** (Node.js 18)
   - Builds React + TypeScript app with Vite
   - Output: Static files in `/dist`

2. **Backend Builder** (Go 1.24)
   - Compiles Go server
   - Output: Single binary

3. **Final Image** (Alpine Linux)
   - Runs Go server on port 8080
   - Serves frontend static files in production
   - Minimal image size (~14MB)

**Services:**
- **Backend API**: Port 8080 (internal)
- **WebSocket**: Real-time game communication
- **Static Files**: Frontend served by Go server in production mode

---

## ⚙️ Environment Variables

Set via `flyctl secrets set`:

| Variable | Required | Description |
|----------|----------|-------------|
| `MONGODB_URI` | ✅ Yes | MongoDB Atlas connection string |
| `JWT_ACCESS_SECRET` | ✅ Yes (set) | JWT signing secret |
| `JWT_REFRESH_SECRET` | ✅ Yes (set) | Refresh token secret |
| `GOOGLE_CLIENT_ID` | ⭕ Optional | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | ⭕ Optional | Google OAuth secret |
| `CHESS_ENV` | Auto | Set to `prod` by fly.toml |

---

## 🔧 Troubleshooting

### App won't start

1. Check secrets are set: `flyctl secrets list --app chessmata`
2. View logs: `flyctl logs --app chessmata`
3. Verify MongoDB connection string is valid

### Certificate not provisioning

1. Verify DNS is configured correctly
2. Check propagation: `dig chessmata.com` or use https://dnschecker.org/
3. Force certificate check: `flyctl certs check chessmata.com --app chessmata`

### Database connection errors

1. Verify MongoDB URI is correct
2. Check MongoDB Atlas network access (whitelist 0.0.0.0/0 for Fly.io)
3. Ensure database user has proper permissions

---

## 💰 Fly.io Costs

**Current configuration:**

- **Machines**: 2 shared-cpu-1x @ 512MB each
  - ~$3-4/month per machine
  - Auto-stop when idle (saves costs)
  - Auto-start on requests

- **IPv4 Address**: $2/month (dedicated)
- **IPv6 Address**: Free

**Estimated monthly cost**: ~$8-10/month

**Free tier includes:**
- Up to 3 shared-cpu-1x machines (256MB)
- 160GB outbound data transfer

---

## 📚 Additional Resources

- **Fly.io Docs**: https://fly.io/docs/
- **MongoDB Atlas**: https://cloud.mongodb.com/
- **Let's Encrypt**: https://letsencrypt.org/
- **API Documentation**: https://chessmata.com/docs (after deployment)

---

## ✅ Post-Deployment Checklist

- [ ] Add DNS records in Namecheap
- [ ] Set `MONGODB_URI` secret
- [ ] (Optional) Set Google OAuth credentials
- [ ] Wait for DNS propagation (5-60 minutes)
- [ ] Verify SSL certificate: https://chessmata.com
- [ ] Test the application
- [ ] Create test user account
- [ ] Start a test game

---

**Deployment completed:** $(date)
**Deployed by:** Claude Code (Anthropic)
