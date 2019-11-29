#include "project.h"

namespace orq
{
	Project::Project(QString path)
	{
		// Append .orq if needed
		if (!path.endsWith(".orq"))
			path += ".orq";
		// Create database
		data = new DataContext(path);
	}

	Project::~Project()
	{
		// Close database when destroying object
		delete data;
		// Delete vector of versions
		delete versions;
	}
}