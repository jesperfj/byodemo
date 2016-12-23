## S3 Buckets for your Heroku Apps on your own AWS Account

Many apps need a place to store files. S3 is by far the most popular service for storing files. One of the benefits of Heroku is that it can set up and handle the full lifecycle of all the "backing services" used by your app, such as a file storage service. But it only works if the service is available as a Heroku Add-on.

Amazon S3 is not available as an add-on, partly because developers are generally happy with buying the service directly from AWS. But nobody likes the hassle of manually configuring and managing S3 buckets to support an application's full lifecycle. That is, creating and destroying buckets for various kinds of test environments in addition to managing the access to a production S3 bucket.

This demo shows a possible solution to this problem.
