# CalDav wrapper for ICA shopping lists

This utility let's you expose your ICA shopping lists over CalDav, which will e.g. give you the possibility to add them to Apple Reminders and say "Hey Siri, add milk to my shopping list" (That's my usecase at least).

There's no good, official way to interact with ICA:s API, so this is derived from intercepting traffic and a hefty `curl` usage.

## Setup

After starting the app it'll launch a server on `localhost:5000`, which both serves caldav and a simple setup flow. Just navigating to `http://localhost:5000` will guide you through setup (AKA logging in to ICA behind the scenes) through BankID. Once this is done, the session will be stored an you can start using caldav.

Test it out by pointing a CalDav client (e.g. Apple Reminders) to `localhost:5000`.

For real deployments there's a `Dockerfile` that should help deploy it in most places, make sure that the `VOLUME` specified there is persisted over launches to avoid having to re-login after restarts.

## Security

There's no security at all provided out of the box, so if you expose this to the internet you should add that yourself.

I'd suggest putting some kind of `Basic auth` in front of it, since that's supported by CalDav AFAIK, and should make it easy to use.

