# Hofweb checker

[Hofweb](https://hofweb.nl) is a Dutch online grocer that specializes in biological produce that is fresh, cheap and grown on local farms. They also sell "2e klas groenten", second class produce, that is only of slightly less quality and even cheaper, but still much better than what you  would find in a local supermarket. My girlfriend s a big fan.

However, it is not always in stock. To make sure she doesn't miss any new offering, I run this script that simply monitors what is available online and that will send an email whenever a new type of produce is made available.

It requires that Chrome or Chromium is installed. Also, it reports status to a Homeassistant instance. See the list of `flag` vars in `main.go` for what arguments are needed to run it.

