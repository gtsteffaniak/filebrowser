# Migration help

It is possible to use the same database as used by filebrowser/filebrowser, 
but you will need to follow the following process:

1. Create a configuration file as mentioned above.
2. Copy your database file from the original filebrowser to the path of
   the new one.
3. Update the configuration file to use the database (under server in
   filebrowser.yml)
4. If you are using docker, update the docker-compose file or docker run
   command to use the config file as described in the install section
   above.
5. If you are not using docker, just make sure you run filebrowser -c
   filebrowser.yml and have a valid filebrowser config.


Note: share links will not work and will need to be re-created after migration.

The filebrowser Quantum application should run with the same user and rules that
you have from the original. But keep in mind the differences that may not work 
the same way, but all user configuration should be available.