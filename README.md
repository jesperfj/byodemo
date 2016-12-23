## S3 Buckets for your Heroku Apps on your own AWS Account

S3 is one of the most common backing services used by Heroku apps. Because S3 is not available as an add-on, developers have to manually manage provisioning, access control and environment configuration when using S3. For a single production app, this is not a huge burden. But once you have a larger organization that regularly build new apps and also practice continuous delivery with ephemeral environments coming and going all the time, it would be nice to take advantage of the automation offered by Heroku Add-ons.

This demo add-on offers exactly that. It works the following way:

1. An administrator representing one or several teams/orgs sets up an AWS account that will contain all the S3 buckets used by all apps. This can be a sub-account in an account family or it can be an IAM user within an account that has sufficient permissions to create S3 buckets and other IAM users.
1. The administraor links one or more Heroku Teams/Orgs to the AWS account.
1. Once the Teams/Orgs have been linked, any developer with permission to create apps in the Team/Org can provision S3 buckets as add-ons.
1. Buckets can be created and destroyed automatically via app.json for Review apps and CI just like any other Heroku Add-on
1. Buckets are owned by the linked AWS account and all usage is billed to that account. The Heroku Add-on plan is (most likely) free. For teams already comfortable with AWS pricing structures, this means no new pricing model for using S3 via Heroku.

## Try it out

You can try it out in just two simple steps:

1. Put yourself in the role of a Heroku Team Admin or someone responsible for AWS accounts. Go to https://byodemo-addon.herokuapp.com and link on or more of your Heroku Teams/Orgs to an AWS account. 
  1. You'll need an AWS Access Key ID and a Secret Access Key. The secret key is encrypted with Fernet in the database. But this being a demo, don't use some all powerful AWS credential.
1. Once the link is created, put yourself in the role of a developer hacking on stuff day-to-day. Maybe you want to build a nice little file manager app. How about starting with the [buckaid sample app](https://github.com/jesperfj/buckaid)? Deploy it with Heroku Button. Remember to deploy it to the team/org that you just configured an AWS account for. Otherwise it won't work.
  1. Marvel at how little you had to do to get some nice sample code working with your very own S3 bucket!
  1. If you get sidetracked and realize you won't have time for this project, just delete your app and your bucket will go away too without leaving unused resources piled up on your AWS invoice.
