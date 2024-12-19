# CalDav wrapper for ICA shopping lists

This utility let's you expose your ICA shopping lists over CalDav, which will e.g. give you the possibility to add them to Apple Reminders and say "Hey Siri, add milk to my shopping list" (That's my usecase at least).

There's no good, official way to interact with ICA:s API, so this is derived from intercepting traffic and a hefty `curl` usage.

## Setup

To get running you need to get a session, this can be found by signing in to [https://www.ica.se](https://www.ica.se), opening the web-inspector and getting the value of the `thSessionId` Cookie. This might seem a bit cumbersom, but it seems like the cookie has a 90 day lifetime, so it'll only be necessary a few times a year.

When you have the cookie we need to put it into the `SESSION_ID` environment variable for it to be picked up, something like.

```shell
SESSION_ID=<cookie-value> go run main.go backend.go
```

And you should be able to test it out by pointing a CalDav client (e.g. Apple Reminders) to `localhost:5000`.

For real deployments there's a `Dockerfile` that should help deploy it in most places, and the `SESSION_ID` can be injected in a suitable way.


## Security

There's no security at all provided out of the box, so if you expose this to the internet you should add that yourself.

I'd suggest putting some kind of `Basic auth` in front of it, since that's supported by CalDav AFAIK, and should make it easy to use.

